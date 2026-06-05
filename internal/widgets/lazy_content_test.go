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
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ovh/kmip-go"
	"github.com/ovh/kmip-go/payloads"
)

// testLoader is a controllable stand-in for the KMIP GetAttributes call.
type testLoader struct {
	mu      sync.Mutex
	calls   map[string]int
	failIDs map[string]bool
	gate    chan struct{} // when non-nil, each load blocks until a value is received
}

func newTestLoader() *testLoader {
	return &testLoader{calls: map[string]int{}, failIDs: map[string]bool{}}
}

func (l *testLoader) callCount(id string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.calls[id]
}

func (l *testLoader) load(id string) (*payloads.GetAttributesResponsePayload, error) {
	l.mu.Lock()
	l.calls[id]++
	gate := l.gate
	fail := l.failIDs[id]
	l.mu.Unlock()
	if gate != nil {
		<-gate
	}
	if fail {
		return nil, errors.New("load failed")
	}
	return namedPayload(id), nil
}

func namedPayload(id string) *payloads.GetAttributesResponsePayload {
	return &payloads.GetAttributesResponsePayload{
		UniqueIdentifier: id,
		Attribute: []kmip.Attribute{{
			AttributeName:  kmip.AttributeNameName,
			AttributeValue: kmip.Name{NameValue: "name-" + id},
		}},
	}
}

func newTestContent(loader func(string) (*payloads.GetAttributesResponsePayload, error)) *lazyContent {
	c := newLazyContent(loader)
	c.requestRedraw = func() {}
	c.startLoader()
	return c
}

// drawRows simulates tview drawing the given data rows: it brackets GetCell calls
// with begin/endFrame, which is what actually enqueues lazy loads.
func drawRows(c *lazyContent, rows ...int) {
	c.beginFrame()
	for _, r := range rows {
		for col := range mobColumns {
			c.GetCell(r, col)
		}
	}
	c.endFrame()
}

// release unblocks exactly one gated in-flight load. It fails the test promptly
// if no worker is waiting on the gate, so a queue/ordering regression surfaces as
// a readable failure instead of hanging until the global test timeout.
func release(t *testing.T, gate chan struct{}) {
	t.Helper()
	select {
	case gate <- struct{}{}:
	case <-time.After(2 * time.Second):
		t.Fatal("no load was waiting on the gate to be released")
	}
}

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}

const nameCol = 2 // ID=0, Type=1, Name=2, ...

func (c *lazyContent) isLoaded(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.loaded[id]
	return ok
}

func TestLazyContentLazyLoadAndCache(t *testing.T) {
	l := newTestLoader()
	c := newTestContent(l.load)

	if got := c.GetColumnCount(); got != mobColumns {
		t.Fatalf("GetColumnCount = %d, want %d", got, mobColumns)
	}
	c.setIDs([]string{"a", "b", "c"})
	if got := c.GetRowCount(); got != 4 {
		t.Fatalf("GetRowCount = %d, want 4 (header + 3)", got)
	}
	if h := c.GetCell(0, 0); h == nil || h.Text != "ID" {
		t.Fatalf("header cell (0,0) = %v, want ID", h)
	}

	// Before load: the id is shown immediately, other columns are placeholders.
	if cell := c.GetCell(1, 0); cell == nil || cell.Text != "a" {
		t.Fatalf("cell (1,0) = %v, want id 'a'", cell)
	}
	if cell := c.GetCell(1, nameCol); cell == nil || cell.Text != "…" {
		t.Fatalf("cell (1,name) = %v, want placeholder", cell)
	}
	// payloadForRow returns a stub (no attributes) while unloaded.
	if p := c.payloadForRow(0); p == nil || p.UniqueIdentifier != "a" || len(p.Attribute) != 0 {
		t.Fatalf("payloadForRow(0) = %v, want stub for 'a'", p)
	}

	// Drawing the row enqueues its load; wait for it to land.
	drawRows(c, 1)
	waitUntil(t, func() bool { return c.isLoaded("a") })

	if cell := c.GetCell(1, nameCol); cell == nil || cell.Text != "name-a" {
		t.Fatalf("cell (1,name) after load = %v, want 'name-a'", cell)
	}
	if p := c.payloadForRow(0); p == nil || len(p.Attribute) != 1 {
		t.Fatalf("payloadForRow(0) after load = %v, want cached payload", p)
	}
	if n := l.callCount("a"); n != 1 {
		t.Fatalf("loader called %d times for 'a', want 1", n)
	}
}

func TestLazyContentDedup(t *testing.T) {
	l := newTestLoader()
	l.gate = make(chan struct{}) // hold the load in-flight
	c := newTestContent(l.load)
	c.setIDs([]string{"a"})

	drawRows(c, 1) // enqueue
	waitUntil(t, func() bool { return l.callCount("a") == 1 })

	// Many redraws while the load is in-flight must not enqueue duplicates.
	for range 20 {
		drawRows(c, 1)
	}
	release(t, l.gate)
	waitUntil(t, func() bool { return c.isLoaded("a") })

	if n := l.callCount("a"); n != 1 {
		t.Fatalf("loader called %d times, want 1 (dedup failed)", n)
	}
}

func TestLazyContentFailureNoRetry(t *testing.T) {
	l := newTestLoader()
	l.failIDs = map[string]bool{"a": true}
	c := newTestContent(l.load)
	c.setIDs([]string{"a"})

	drawRows(c, 1)
	waitUntil(t, func() bool { return c.GetCell(1, nameCol).Text == "!" })

	// Subsequent draws must not retry a failed id.
	for range 10 {
		drawRows(c, 1)
	}
	time.Sleep(30 * time.Millisecond)
	if n := l.callCount("a"); n != 1 {
		t.Fatalf("loader called %d times for failed id, want 1 (no retry)", n)
	}
	// The id is still shown even on failure.
	if cell := c.GetCell(1, 0); cell.Text != "a" {
		t.Fatalf("failed cell (1,0) = %q, want id 'a'", cell.Text)
	}
}

func TestLazyContentGenInvalidation(t *testing.T) {
	l := newTestLoader()
	l.gate = make(chan struct{})
	c := newTestContent(l.load)
	c.setIDs([]string{"x"})

	drawRows(c, 1)
	waitUntil(t, func() bool { return l.callCount("x") == 1 }) // worker is in loader("x")

	c.setIDs([]string{"y"}) // bump gen while the old load is in-flight
	release(t, l.gate)      // release the now-stale load

	time.Sleep(30 * time.Millisecond)
	if c.isLoaded("x") {
		t.Fatal("stale-generation result for 'x' must not be cached after setIDs")
	}
}

func TestLazyContentRemoveAndPut(t *testing.T) {
	l := newTestLoader()
	c := newTestContent(l.load)
	c.setIDs([]string{"a", "b", "c"})

	c.put(namedPayload("b"))
	if cell := c.GetCell(2, nameCol); cell.Text != "name-b" { // row 2 -> "b"
		t.Fatalf("cell (2,name) after put = %q, want 'name-b'", cell.Text)
	}
	// put for an id absent from the row set is a no-op.
	c.put(namedPayload("zzz"))
	if c.isLoaded("zzz") {
		t.Fatal("put for unknown id should be ignored")
	}

	c.remove("b")
	if got := c.GetRowCount(); got != 3 { // header + a + c
		t.Fatalf("GetRowCount after remove = %d, want 3", got)
	}
	if cell := c.GetCell(2, 0); cell.Text != "c" { // row 2 is now "c"
		t.Fatalf("cell (2,0) after remove = %q, want 'c'", cell.Text)
	}
	if c.isLoaded("b") {
		t.Fatal("removed id 'b' should be evicted from cache")
	}
}

func TestLazyContentOnlyVisibleRowsLoad(t *testing.T) {
	l := newTestLoader()
	c := newTestContent(l.load)
	c.setIDs([]string{"a", "b", "c", "d", "e"})

	// Nothing has been drawn yet, so nothing should be fetched.
	time.Sleep(30 * time.Millisecond)
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		if n := l.callCount(id); n != 0 {
			t.Fatalf("id %q fetched %d times before being drawn, want 0", id, n)
		}
	}

	// Draw only rows 1 and 2 (ids a, b): only those load, the rest stay untouched.
	drawRows(c, 1, 2)
	waitUntil(t, func() bool { return c.isLoaded("a") && c.isLoaded("b") })
	for _, id := range []string{"c", "d", "e"} {
		if n := l.callCount(id); n != 0 {
			t.Fatalf("undrawn id %q fetched %d times, want 0", id, n)
		}
	}
}

func TestLazyContentDropsScrolledOffRows(t *testing.T) {
	l := newTestLoader()
	l.gate = make(chan struct{})
	c := newTestContent(l.load)
	c.setIDs([]string{"a", "b", "c", "d", "e", "f"})

	// First viewport: rows 1-3 (a,b,c). The single worker grabs "a" and blocks in
	// the loader, leaving b,c queued behind it.
	drawRows(c, 1, 2, 3)
	waitUntil(t, func() bool { return l.callCount("a") == 1 })

	// Scroll away before b,c are fetched: a new frame showing rows 5,6 (e,f)
	// rebuilds the queue from the current viewport and drops b,c.
	drawRows(c, 5, 6)

	// Release the in-flight "a"; the worker then drains the *new* queue (e,f).
	release(t, l.gate) // a
	release(t, l.gate) // e
	release(t, l.gate) // f
	waitUntil(t, func() bool { return c.isLoaded("e") && c.isLoaded("f") })

	for _, id := range []string{"b", "c"} {
		if n := l.callCount(id); n != 0 {
			t.Fatalf("scrolled-off id %q fetched %d times, want 0 (should be dropped)", id, n)
		}
	}
}

func TestLazyContentPutNotClobberedByInflightLoad(t *testing.T) {
	l := newTestLoader()
	l.gate = make(chan struct{})
	c := newTestContent(l.load)
	c.setIDs([]string{"a"})

	// Worker grabs "a" and blocks in the loader (which would return name "name-a").
	drawRows(c, 1)
	waitUntil(t, func() bool { return l.callCount("a") == 1 })

	// Fresher data lands via put (e.g. just after an Activate) while the load is
	// still in flight.
	c.put(&payloads.GetAttributesResponsePayload{
		UniqueIdentifier: "a",
		Attribute: []kmip.Attribute{{
			AttributeName:  kmip.AttributeNameName,
			AttributeValue: kmip.Name{NameValue: "fresh"},
		}},
	})
	if cell := c.GetCell(1, nameCol); cell.Text != "fresh" {
		t.Fatalf("cell after put = %q, want 'fresh'", cell.Text)
	}

	// Let the now-stale in-flight load complete: it must NOT overwrite the put.
	release(t, l.gate)
	time.Sleep(30 * time.Millisecond)
	if cell := c.GetCell(1, nameCol); cell.Text != "fresh" {
		t.Fatalf("cell after stale load completed = %q, want 'fresh' (put was clobbered)", cell.Text)
	}
}

func TestLazyContentLoaderPanicIsContained(t *testing.T) {
	panicking := func(string) (*payloads.GetAttributesResponsePayload, error) {
		panic("boom")
	}
	c := newTestContent(panicking)
	c.setIDs([]string{"a", "b"})

	// A panic during a load must be contained: the row shows the failure marker...
	drawRows(c, 1)
	waitUntil(t, func() bool { return c.GetCell(1, nameCol).Text == "!" })

	// ...and the worker must still be alive to serve the next row.
	drawRows(c, 2)
	waitUntil(t, func() bool { return c.GetCell(2, nameCol).Text == "!" })
}

func TestBuildRowCellsToleratesBadTypes(t *testing.T) {
	const sizeCol = 4 // ID=0, Type=1, Name=2, Algorithm=3, Size=4, ...
	// CryptographicLength carries the wrong Go type; Name is well-formed.
	v := &payloads.GetAttributesResponsePayload{
		UniqueIdentifier: "a",
		Attribute: []kmip.Attribute{
			{AttributeName: kmip.AttributeNameCryptographicLength, AttributeValue: "not-an-int32"},
			{AttributeName: kmip.AttributeNameName, AttributeValue: kmip.Name{NameValue: "ok"}},
		},
	}
	cells, _ := buildRowCells(v) // must not panic
	if cells[nameCol].Text != "ok" {
		t.Fatalf("name cell = %q, want 'ok'", cells[nameCol].Text)
	}
	if cells[sizeCol].Text != "" {
		t.Fatalf("size cell = %q, want empty (bad type ignored)", cells[sizeCol].Text)
	}
}

func TestLazyContentAgeRecomputedOnDraw(t *testing.T) {
	l := newTestLoader()
	c := newTestContent(l.load)
	c.setIDs([]string{"a"})

	// Load "a" with an InitialDate ~30s ago: Age starts at "<1m".
	c.put(&payloads.GetAttributesResponsePayload{
		UniqueIdentifier: "a",
		Attribute: []kmip.Attribute{{
			AttributeName:  kmip.AttributeNameInitialDate,
			AttributeValue: time.Now().Add(-30 * time.Second),
		}},
	})
	if cell := c.GetCell(1, ageCol); cell.Text != "<1m" {
		t.Fatalf("initial age = %q, want '<1m'", cell.Text)
	}

	// Simulate time passing by ageing the cached InitialDate. A redraw (GetCell)
	// must recompute Age from the stored date rather than serve the frozen "<1m".
	c.mu.Lock()
	c.loaded["a"].date = time.Now().Add(-49 * time.Hour)
	c.mu.Unlock()
	if cell := c.GetCell(1, ageCol); cell.Text != "2d" {
		t.Fatalf("age after time passed = %q, want '2d' (recomputed, not frozen)", cell.Text)
	}
}

func TestLazyContentRemoveDropsQueuedFetch(t *testing.T) {
	l := newTestLoader()
	l.gate = make(chan struct{})
	c := newTestContent(l.load)
	c.setIDs([]string{"a", "b"})

	// Worker grabs "a" and blocks; "b" stays queued behind it.
	drawRows(c, 1, 2)
	waitUntil(t, func() bool { return l.callCount("a") == 1 })

	// Remove "b" before the worker reaches it: it must be dropped from the queue,
	// not fetched.
	c.remove("b")

	release(t, l.gate) // finish "a"; the worker then finds the queue empty
	waitUntil(t, func() bool { return c.isLoaded("a") })
	time.Sleep(30 * time.Millisecond)
	if n := l.callCount("b"); n != 0 {
		t.Fatalf("removed-while-queued id 'b' fetched %d times, want 0", n)
	}
	if c.isLoaded("b") {
		t.Fatal("removed id 'b' must not be cached")
	}
}

func TestLazyContentLoadErrDistinguishesFailedFromLoading(t *testing.T) {
	l := newTestLoader()
	l.failIDs = map[string]bool{"b": true}
	c := newTestContent(l.load)
	c.setIDs([]string{"a", "b"})

	// Nothing has been drawn yet, so neither row has loaded or failed.
	if err := c.loadErr(0); err != nil {
		t.Fatalf("loadErr(0) before any load = %v, want nil (still loading)", err)
	}

	drawRows(c, 1, 2)
	waitUntil(t, func() bool { return c.isLoaded("a") })
	waitUntil(t, func() bool { return c.GetCell(2, nameCol).Text == "!" })

	// "a" loaded fine -> no error; "b" failed -> loadErr surfaces the error so the
	// attributes panel can show it instead of a perpetual "Loading…".
	if err := c.loadErr(0); err != nil {
		t.Fatalf("loadErr(0) for a loaded row = %v, want nil", err)
	}
	if err := c.loadErr(1); err == nil {
		t.Fatal("loadErr(1) for a failed row = nil, want the load error")
	}
	// Out-of-range is nil, not a panic.
	if err := c.loadErr(99); err != nil {
		t.Fatalf("loadErr(99) out of range = %v, want nil", err)
	}
}

func TestLazyContentPlaceholderCellsCached(t *testing.T) {
	l := newTestLoader()
	c := newTestContent(l.load)
	c.setIDs([]string{"a"})

	// GetCell outside a draw frame doesn't enqueue a load, so "a" stays unloaded.
	// The same unloaded row must return the identical cached cell across calls
	// rather than allocating a fresh placeholder every time it is drawn.
	first := c.GetCell(1, nameCol)
	second := c.GetCell(1, nameCol)
	if first != second {
		t.Fatal("placeholder cell for an unloaded row should be cached across calls")
	}
	if first.Text != "…" {
		t.Fatalf("unloaded placeholder text = %q, want '…'", first.Text)
	}
}

func TestLazyContentPlaceholderRebuildsOnFailure(t *testing.T) {
	l := newTestLoader()
	l.failIDs = map[string]bool{"a": true}
	c := newTestContent(l.load)
	c.setIDs([]string{"a"})

	if loading := c.GetCell(1, nameCol); loading.Text != "…" { // caches the "…" variant
		t.Fatalf("placeholder before load = %q, want '…'", loading.Text)
	}

	drawRows(c, 1) // enqueue; the load fails
	waitUntil(t, func() bool { return c.GetCell(1, nameCol).Text == "!" })

	// The cached loading placeholder must have been invalidated and rebuilt as the
	// failed ("!") variant, not left showing the stale "…".
	if failed := c.GetCell(1, nameCol); failed.Text != "!" {
		t.Fatalf("placeholder after failure = %q, want '!'", failed.Text)
	}
}
