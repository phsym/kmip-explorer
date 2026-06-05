// Copyright 2025 Pierre-Henri Symoneaux
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package widgets

import (
	"fmt"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/ovh/kmip-go"
	"github.com/ovh/kmip-go/payloads"
	"github.com/ovh/kmip-go/ttlv"
	"github.com/rivo/tview"
)

// mobColumns is the number of columns rendered for each managed object:
// ID, Type, Name, Algorithm, Size, State, Age.
const mobColumns = 7

// ageCol is the index of the "Age" column. Its text is recomputed on every draw
// from the row's cached InitialDate (see GetCell) so it doesn't freeze at the
// value it had when the row was first loaded.
const ageCol = mobColumns - 1

// loadedRow holds everything cached for a fully loaded row, all written and
// cleared as a unit: the raw details (used by the attributes panel), the
// precomputed table cells, and the InitialDate used to recompute the
// time-relative Age column on each draw (zero = unknown).
type loadedRow struct {
	payload *payloads.GetAttributesResponsePayload
	cells   [mobColumns]*tview.TableCell
	date    time.Time
}

// lazyContent implements tview.TableContent. It holds the ordered list of object
// IDs known immediately after a Locate, and lazily fetches each object's details
// (GetAttributes) only when its row is drawn on screen, caching the result so a
// row is never fetched twice. tview.Table.Draw only calls GetCell for the visible
// rows (plus the fixed header and the selected row), so GetCell is the natural
// "this row is on screen" trigger. GetCell runs on the UI/draw goroutine and must
// never block: it returns a placeholder and enqueues a background load.
type lazyContent struct {
	tview.TableContentReadOnly

	mu           sync.Mutex
	ids          []string                                // row r (1-based) -> ids[r-1]; header is row 0
	loaded       map[string]*loadedRow                   // fully loaded rows: details + precomputed cells + InitialDate
	placeholders map[string][mobColumns]*tview.TableCell // cached "…"/"!" cells for not-yet-loaded rows, parallel to loaded
	inflight     map[string]struct{}                     // currently being fetched (dedup)
	failed       map[string]error                        // load error per id (absent = not failed); cleared on refresh
	queue        []string                                // ids to load for the current viewport, top-to-bottom
	gen          uint64                                  // bumped on setIDs; stale results are discarded

	// frame collects the ids drawn during the current frame, in top-to-bottom
	// order. endFrame turns it into the work queue, so the loader only fetches
	// rows that are actually on screen and always starts from the top.
	frame    []string
	frameSet map[string]struct{}
	framing  bool

	wake          chan struct{} // buffered(1) signal to wake the loader
	loader        func(id string) (*payloads.GetAttributesResponsePayload, error)
	requestRedraw func() // coalesced redraw, set by MobTable

	header [mobColumns]*tview.TableCell
}

func newLazyContent(loader func(id string) (*payloads.GetAttributesResponsePayload, error)) *lazyContent {
	c := &lazyContent{
		loaded:       map[string]*loadedRow{},
		placeholders: map[string][mobColumns]*tview.TableCell{},
		inflight:     map[string]struct{}{},
		failed:       map[string]error{},
		frameSet:     map[string]struct{}{},
		wake:         make(chan struct{}, 1),
		loader:       loader,
	}
	hdrStyle := tcell.StyleDefault.Bold(true)
	for i, txt := range []string{"ID", "Type", "Name", "Algorithm", "Size", "State", "Age"} {
		c.header[i] = newStyledCell(txt, hdrStyle)
	}
	return c
}

// --- tview.TableContent ---

func (c *lazyContent) GetRowCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.ids) + 1
}

func (c *lazyContent) GetColumnCount() int { return mobColumns }

func (c *lazyContent) GetCell(row, column int) *tview.TableCell {
	if column < 0 || column >= mobColumns {
		return nil
	}
	if row == 0 {
		return c.header[column]
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	i := row - 1
	if i < 0 || i >= len(c.ids) {
		return nil
	}
	id := c.ids[i]
	if lr, ok := c.loaded[id]; ok {
		// Age is time-relative, so refresh it from the cached InitialDate on every
		// draw rather than serving the value frozen at load time.
		if column == ageCol && !lr.date.IsZero() {
			lr.cells[ageCol].SetText(formatAge(time.Since(lr.date)))
		}
		return lr.cells[column]
	}
	_, failed := c.failed[id]
	if !failed {
		c.recordFrameLocked(id)
	}
	// Placeholder cells are cached per id (parallel to c.loaded) so a row that
	// stays on screen unloaded — or one that permanently failed — is not
	// re-allocated on every frame. The cache is invalidated wherever the row's
	// state changes: storeLocked (loaded), the failed transitions (rebuild as
	// "!"), remove, and setIDs.
	ph, ok := c.placeholders[id]
	if !ok {
		ph = buildPlaceholderCells(id, failed)
		c.placeholders[id] = ph
	}
	return ph[column]
}

// --- model mutation (all called from the UI goroutine) ---

// setIDs replaces the row set and invalidates everything else. Bumping gen makes
// any in-flight load from the previous set discard its result on completion.
func (c *lazyContent) setIDs(ids []string) {
	c.mu.Lock()
	c.ids = ids
	c.gen++
	c.loaded = map[string]*loadedRow{}
	c.placeholders = map[string][mobColumns]*tview.TableCell{}
	c.inflight = map[string]struct{}{}
	c.failed = map[string]error{}
	c.queue = nil
	c.frame = nil
	c.frameSet = map[string]struct{}{}
	c.framing = false
	c.mu.Unlock()
}

// put overwrites the cached details for an id already present in the row set.
func (c *lazyContent) put(o *payloads.GetAttributesResponsePayload) {
	if o == nil {
		return
	}
	// Build the cells before locking, matching loadOne, so buildRowCells never
	// runs while c.mu is held (see storeLocked).
	cells, date := buildRowCells(o)
	c.mu.Lock()
	defer c.mu.Unlock()
	if !slices.Contains(c.ids, o.UniqueIdentifier) {
		return
	}
	c.storeLocked(o, cells, date)
	delete(c.failed, o.UniqueIdentifier)
	delete(c.inflight, o.UniqueIdentifier)
}

func (c *lazyContent) remove(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if i := slices.Index(c.ids, id); i >= 0 {
		c.ids = slices.Delete(c.ids, i, i+1)
	}
	// Drop any pending queue entry so the worker doesn't fetch a row that no
	// longer exists. A load already in flight is handled by loadOne's in-flight
	// check (remove clears the marker below, so its result is dropped).
	c.queue = slices.DeleteFunc(c.queue, func(q string) bool { return q == id })
	delete(c.loaded, id)
	delete(c.placeholders, id)
	delete(c.failed, id)
	delete(c.inflight, id)
}

// payloadForRow returns the cached details for a data row, or a stub carrying just
// the id when not yet loaded (nil only for the header or an out-of-range row). The
// stub lets operations (which only need the UniqueIdentifier) work before details load.
func (c *lazyContent) payloadForRow(i int) *payloads.GetAttributesResponsePayload {
	c.mu.Lock()
	defer c.mu.Unlock()
	if i < 0 || i >= len(c.ids) {
		return nil
	}
	id := c.ids[i]
	if lr, ok := c.loaded[id]; ok {
		return lr.payload
	}
	return &payloads.GetAttributesResponsePayload{UniqueIdentifier: id}
}

// loadErr returns the error that failed data row i's load, or nil if the row
// loaded successfully, is still loading, or is out of range. The attributes
// panel uses it to tell a failed row apart from one that is merely still loading
// (both look like an empty payload via payloadForRow).
func (c *lazyContent) loadErr(i int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if i < 0 || i >= len(c.ids) {
		return nil
	}
	return c.failed[c.ids[i]]
}

// --- loading ---

// beginFrame starts collecting the ids drawn this frame. MobTable.Draw calls it
// before the underlying table draws (and therefore before GetCell runs).
func (c *lazyContent) beginFrame() {
	c.mu.Lock()
	c.frame = c.frame[:0]
	clear(c.frameSet)
	c.framing = true
	c.mu.Unlock()
}

// recordFrameLocked appends an on-screen, not-yet-loaded id to the current frame,
// preserving the top-to-bottom order in which GetCell visits rows. Caller holds c.mu.
func (c *lazyContent) recordFrameLocked(id string) {
	if !c.framing {
		return
	}
	if _, ok := c.frameSet[id]; ok {
		return
	}
	c.frameSet[id] = struct{}{}
	c.frame = append(c.frame, id)
}

// endFrame rebuilds the work queue from the rows just drawn, so the loader always
// works the current viewport top-to-bottom and rows scrolled off screen are
// dropped rather than fetched later.
func (c *lazyContent) endFrame() {
	c.mu.Lock()
	c.framing = false
	c.queue = make([]string, 0, len(c.frame))
	for _, id := range c.frame {
		if c.needsLoadLocked(id) {
			c.queue = append(c.queue, id)
		}
	}
	hasWork := len(c.queue) > 0
	c.mu.Unlock()
	if hasWork {
		select {
		case c.wake <- struct{}{}:
		default:
		}
	}
}

// needsLoadLocked reports whether id still needs fetching — not already loaded,
// not in flight, and not failed. endFrame uses it to build an accurate work queue
// (and wake signal); popLocked re-checks with it because a row's state can change
// between being queued and being popped. Caller holds c.mu.
func (c *lazyContent) needsLoadLocked(id string) bool {
	if _, ok := c.loaded[id]; ok {
		return false
	}
	if _, inf := c.inflight[id]; inf {
		return false
	}
	if _, bad := c.failed[id]; bad {
		return false
	}
	return true
}

// popLocked takes the next id from the front of the queue (the topmost on-screen
// row). Caller holds c.mu.
func (c *lazyContent) popLocked() (id string, gen uint64, ok bool) {
	for len(c.queue) > 0 {
		id = c.queue[0]
		c.queue = c.queue[1:]
		if !c.needsLoadLocked(id) {
			continue
		}
		c.inflight[id] = struct{}{}
		return id, c.gen, true
	}
	return "", 0, false
}

// storeLocked caches an object's details together with its precomputed cells. The
// cells are built by the caller so that buildRowCells (which could in theory panic
// on a malformed payload) never runs while c.mu is held. Caller holds c.mu.
func (c *lazyContent) storeLocked(v *payloads.GetAttributesResponsePayload, cells [mobColumns]*tview.TableCell, initialDate time.Time) {
	c.loaded[v.UniqueIdentifier] = &loadedRow{payload: v, cells: cells, date: initialDate}
	delete(c.placeholders, v.UniqueIdentifier) // now served from c.loaded
}

// startLoader runs the single background worker. The KMIP transport serializes
// requests anyway, so one worker is optimal — and the single-worker invariant is
// also what keeps loadOne's in-flight dedup correct.
func (c *lazyContent) startLoader() {
	go func() {
		for range c.wake {
			for {
				c.mu.Lock()
				id, gen, ok := c.popLocked()
				c.mu.Unlock()
				if !ok {
					break
				}
				c.loadOne(id, gen)
			}
		}
	}()
}

// loadOne fetches one object's details and caches them, unless the load was
// superseded while it was in flight. A load is superseded when setIDs replaced the
// row set (gen bumped) or put/remove cleared this id's in-flight marker — the
// latter means fresher data (or a deletion) already won, so a stale fetch must not
// clobber it. This is correct because there is a single worker: only the worker
// marks a row in-flight, so once we observe the marker cleared, nothing re-set it
// underneath us.
//
// A panic (e.g. an unexpected attribute type) is contained: the row is marked
// failed and the worker keeps serving the rest of the queue instead of taking the
// whole app down. buildRowCells runs off-lock so the recover can never fire while
// holding c.mu (which would deadlock completeLoad's re-lock).
func (c *lazyContent) loadOne(id string, gen uint64) {
	defer func() {
		if r := recover(); r != nil {
			c.completeLoad(id, gen, nil, [mobColumns]*tview.TableCell{}, time.Time{}, fmt.Errorf("loading attributes panicked: %v", r))
		}
	}()
	v, err := c.loader(id)
	var (
		cells [mobColumns]*tview.TableCell
		date  time.Time
	)
	if err == nil {
		cells, date = buildRowCells(v)
	}
	c.completeLoad(id, gen, v, cells, date, err)
}

// completeLoad finalizes one load attempt: it clears the in-flight marker and,
// unless the load was superseded, applies the outcome — caching the details on
// success or recording the error on failure — then requests a redraw. The normal
// completion path and the panic-recovery path both funnel through here so they
// clean up identically.
func (c *lazyContent) completeLoad(id string, gen uint64, v *payloads.GetAttributesResponsePayload, cells [mobColumns]*tview.TableCell, date time.Time, err error) {
	c.mu.Lock()
	_, current := c.inflight[id]
	delete(c.inflight, id)
	superseded := gen != c.gen || !current
	switch {
	case superseded:
		// A newer setIDs/put/remove already won; drop this result.
	case err != nil:
		c.failed[id] = err
		delete(c.placeholders, id) // rebuild as the failed ("!") variant
	default:
		c.storeLocked(v, cells, date)
	}
	c.mu.Unlock()
	if !superseded && c.requestRedraw != nil {
		c.requestRedraw()
	}
}

// --- cell rendering ---

// newStyledCell builds a table cell with the standard column styling shared by
// every row: the given style, its reverse for the selected row, and equal-width
// expansion.
func newStyledCell(txt string, style tcell.Style) *tview.TableCell {
	return tview.NewTableCell(txt).SetStyle(style).SetSelectedStyle(style.Reverse(true)).SetExpansion(1)
}

func placeholderCell(id string, column int, marker string, color tcell.Color) *tview.TableCell {
	txt := marker
	style := tcell.StyleDefault.Foreground(color)
	if column == 0 {
		txt = id
		style = tcell.StyleDefault
	}
	return newStyledCell(txt, style)
}

// buildPlaceholderCells builds the seven cells shown for a row whose details
// aren't available yet: "…" while the load is pending, "!" once it has failed.
// The id column always shows the id so the row stays identifiable in either state.
func buildPlaceholderCells(id string, failed bool) [mobColumns]*tview.TableCell {
	marker, color := "…", tcell.ColorDarkGray
	if failed {
		marker, color = "!", tcell.ColorIndianRed
	}
	var cells [mobColumns]*tview.TableCell
	for col := range cells {
		cells[col] = placeholderCell(id, col, marker, color)
	}
	return cells
}

// formatAge renders a duration as the compact age shown in the Age column.
func formatAge(d time.Duration) string {
	switch {
	case d >= 24*time.Hour:
		return strconv.Itoa(int(d/(24*time.Hour))) + "d"
	case d > time.Hour:
		return strconv.Itoa(int(d/time.Hour)) + "h" + strconv.Itoa(int((d%time.Hour)/time.Minute)) + "m"
	case d > time.Minute:
		return strconv.Itoa(int(d/time.Minute)) + "m"
	default:
		return "<1m"
	}
}

// buildRowCells extracts the displayed attributes once and builds all seven cells.
// It also returns the row's InitialDate (zero if absent) so the caller can refresh
// the time-relative Age column on later draws instead of freezing it here.
func buildRowCells(v *payloads.GetAttributesResponsePayload) ([mobColumns]*tview.TableCell, time.Time) {
	var (
		otype       kmip.ObjectType
		size        string
		state       kmip.State
		name        string
		alg         string
		age         string
		initialDate time.Time
	)
	// Type assertions use the comma-ok form throughout: a server returning an
	// unexpected Go type for an attribute leaves that column blank rather than
	// panicking the background loader (which would crash the whole app).
	for _, attr := range v.Attribute {
		switch attr.AttributeName {
		case kmip.AttributeNameCryptographicLength:
			if l, ok := attr.AttributeValue.(int32); ok {
				size = strconv.Itoa(int(l))
			}
		case kmip.AttributeNameObjectType:
			if t, ok := attr.AttributeValue.(kmip.ObjectType); ok {
				otype = t
			}
		case kmip.AttributeNameState:
			if s, ok := attr.AttributeValue.(kmip.State); ok {
				state = s
			}
		case kmip.AttributeNameName:
			if attr.AttributeIndex != nil && *attr.AttributeIndex != 0 {
				continue
			}
			if n, ok := attr.AttributeValue.(kmip.Name); ok {
				name = n.NameValue
			}
		case kmip.AttributeNameCryptographicAlgorithm:
			if a, ok := attr.AttributeValue.(kmip.CryptographicAlgorithm); ok {
				alg = ttlv.EnumStr(a)
			}
		case kmip.AttributeNameInitialDate:
			t, ok := attr.AttributeValue.(time.Time)
			if !ok {
				continue
			}
			initialDate = t
			age = formatAge(time.Since(t))
		}
	}
	style := tcell.StyleDefault
	switch state {
	case kmip.StateActive:
		style = style.Foreground(tcell.ColorBlue)
	case kmip.StateDeactivated:
		style = style.Foreground(tcell.ColorDarkGrey)
	case kmip.StateCompromised:
		style = style.Foreground(tcell.ColorIndianRed)
	case kmip.StateDestroyed:
		style = style.Foreground(tcell.ColorDarkGrey).StrikeThrough(true)
	case kmip.StateDestroyedCompromised:
		style = style.Foreground(tcell.ColorIndianRed).StrikeThrough(true)
	}
	values := [mobColumns]string{
		v.UniqueIdentifier,
		ttlv.EnumStr(otype),
		name,
		alg,
		size,
		ttlv.EnumStr(state),
		age,
	}
	var cells [mobColumns]*tview.TableCell
	for i, txt := range values {
		cells[i] = newStyledCell(txt, style)
	}
	return cells, initialDate
}
