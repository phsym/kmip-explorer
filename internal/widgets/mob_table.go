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
	"sync/atomic"

	"github.com/ovh/kmip-go/payloads"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type MobTable struct {
	*tview.Table
	content         *lazyContent
	app             *tview.Application
	redrawPending   atomic.Bool
	onSelection     func(*payloads.GetAttributesResponsePayload)
	onSelected      func(*payloads.GetAttributesResponsePayload)
	onContentUpdate func()
	title           string
}

// NewMobTable builds the managed-objects table. loader fetches a single object's
// details (it is called from a background goroutine); app is used to marshal
// redraws back onto the UI thread when lazily-loaded details arrive.
func NewMobTable(loader func(id string) (*payloads.GetAttributesResponsePayload, error), app *tview.Application) *MobTable {
	mtb := &MobTable{
		app:     app,
		content: newLazyContent(loader),
	}
	mtb.content.requestRedraw = mtb.requestRedraw

	mtb.Table = tview.NewTable().
		SetSelectable(true, false)
	mtb.Table.SetContent(mtb.content)
	mtb.Table.SetBorder(true)
	mtb.Table.SetFixed(1, 0)
	mtb.Table.Select(0, 0)
	mtb.Table.SetSelectionChangedFunc(func(row, column int) {
		mtb.updateTitle()
		if mtb.onSelection != nil {
			mtb.onSelection(mtb.GetSelection())
		}
	})
	mtb.Table.SetSelectedFunc(func(row, column int) {
		if mtb.onSelected != nil {
			mtb.onSelected(mtb.GetSelection())
		}
	})

	mtb.content.startLoader()

	return mtb
}

// Draw brackets the underlying table draw with begin/endFrame so the content
// learns exactly which rows are on screen this frame and loads only those,
// top-to-bottom.
func (mtb *MobTable) Draw(screen tcell.Screen) {
	mtb.content.beginFrame()
	defer mtb.content.endFrame()
	mtb.Table.Draw(screen)
}

// requestRedraw coalesces detail-load completions into at most one queued redraw.
// The queued closure runs on the UI thread: it refreshes the attributes panel for
// the current selection, and the implicit Draw re-reads the now-cached cells.
func (mtb *MobTable) requestRedraw() {
	if mtb.app == nil {
		return
	}
	if !mtb.redrawPending.CompareAndSwap(false, true) {
		return
	}
	mtb.app.QueueUpdateDraw(func() {
		mtb.redrawPending.Store(false)
		if mtb.onContentUpdate != nil {
			mtb.onContentUpdate()
		}
	})
}

func (mtb *MobTable) SetTitle(title string) {
	mtb.title = title
	mtb.updateTitle()
}

func (mtb *MobTable) updateTitle() {
	row, _ := mtb.Table.GetSelection()
	total := mtb.Table.GetRowCount() - 1
	title := fmt.Sprintf("%s [%d/%d]", mtb.title, row, total)
	mtb.Table.SetTitle(title)
}

func (mtb *MobTable) contentUpdated() {
	mtb.updateTitle()
	if mtb.onContentUpdate != nil {
		mtb.onContentUpdate()
	}
}

func (mtb *MobTable) Clear(scrollToBeginning bool) {
	if scrollToBeginning {
		mtb.Table.Select(0, 0)
		mtb.Table.ScrollToBeginning()
	}
	mtb.content.setIDs(nil)
	mtb.contentUpdated()
}

// SetIDs renders one row per object id immediately (the id column is filled, the
// rest show placeholders). Per-object details are loaded lazily for visible rows.
func (mtb *MobTable) SetIDs(ids []string) {
	mtb.content.setIDs(ids)
	mtb.contentUpdated()
}

func (mtb *MobTable) RemoveObject(id string) {
	mtb.content.remove(id)
	mtb.contentUpdated()
}

func (mtb *MobTable) UpdateObject(object *payloads.GetAttributesResponsePayload) {
	mtb.content.put(object)
	mtb.contentUpdated()
}

func (mtb *MobTable) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return mtb.Table.WrapInputHandler(func(ek *tcell.EventKey, f func(p tview.Primitive)) {
		if ek.Key() == tcell.KeyESC {
			mtb.Table.Select(0, 0)
			mtb.Table.ScrollToBeginning()
			return
		}
		mtb.Table.InputHandler()(ek, f)
	})
}

// GetSelection returns the selected object's details, or a stub carrying just the
// id when details haven't loaded yet (nil only when no data row is selected).
func (tb *MobTable) GetSelection() *payloads.GetAttributesResponsePayload {
	row, _ := tb.Table.GetSelection()
	if row <= 0 {
		return nil
	}
	return tb.content.payloadForRow(row - 1)
}

// SelectionError returns the load error for the selected row, or nil if its
// details loaded successfully, are still loading, or no data row is selected.
// Lets the caller distinguish a failed row from one that is merely still loading.
func (tb *MobTable) SelectionError() error {
	row, _ := tb.Table.GetSelection()
	if row <= 0 {
		return nil
	}
	return tb.content.loadErr(row - 1)
}

func (tb *MobTable) OnSelectionChanged(cb func(*payloads.GetAttributesResponsePayload)) *MobTable {
	tb.onSelection = cb
	return tb
}
func (tb *MobTable) OnSelected(cb func(*payloads.GetAttributesResponsePayload)) *MobTable {
	tb.onSelected = cb
	return tb
}

func (tb *MobTable) OnContentUpdate(cb func()) *MobTable {
	tb.onContentUpdate = cb
	return tb
}
