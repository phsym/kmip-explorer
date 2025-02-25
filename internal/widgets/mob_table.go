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
	"time"

	"github.com/ovh/kmip-go/ttlv"

	"github.com/ovh/kmip-go"
	"github.com/ovh/kmip-go/payloads"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type MobTable struct {
	*tview.Table
	objects         []*payloads.GetAttributesResponsePayload
	onSelection     func(*payloads.GetAttributesResponsePayload)
	onSelected      func(*payloads.GetAttributesResponsePayload)
	onContentUpdate func()
	title           string
}

func NewMobTable() *MobTable {
	mtb := &MobTable{}
	newHdrCell := func(txt string) *tview.TableCell {
		style := tcell.StyleDefault.Bold(true)
		return tview.NewTableCell(txt).SetStyle(style).SetSelectedStyle(style.Reverse(true)).SetExpansion(1)
	}

	mtb.Table = tview.NewTable().
		SetSelectable(true, false).
		SetCell(0, 0, newHdrCell("ID")).
		SetCell(0, 1, newHdrCell("Type")).
		SetCell(0, 2, newHdrCell("Name")).
		SetCell(0, 3, newHdrCell("Algorithm")).
		SetCell(0, 4, newHdrCell("Size")).
		SetCell(0, 5, newHdrCell("State")).
		SetCell(0, 6, newHdrCell("Age"))
	mtb.Table.SetBorder(true)
	mtb.Table.SetFixed(1, 0)
	mtb.Table.Select(0, 0)
	mtb.Table.SetSelectionChangedFunc(func(row, column int) {
		mtb.updateTitle()
		if mtb.onSelection != nil {
			var obj *payloads.GetAttributesResponsePayload
			if row > 0 && row <= len(mtb.objects) {
				obj = mtb.objects[row-1]
			}
			mtb.onSelection(obj)
		}
	})
	mtb.Table.SetSelectedFunc(func(row, column int) {
		if mtb.onSelected != nil {
			var obj *payloads.GetAttributesResponsePayload
			if row > 0 && row <= len(mtb.objects) {
				obj = mtb.objects[row-1]
			}
			mtb.onSelected(obj)
		}
	})

	return mtb
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

func (mtb *MobTable) rebuildTable() {
	mtb.Clear(false)
	for i, v := range mtb.objects {
		var (
			otype kmip.ObjectType
			size  string
			state kmip.State
			name  string
			alg   string
			age   string
		)
		for _, attr := range v.Attribute {
			switch attr.AttributeName {
			case kmip.AttributeNameCryptographicLength:
				size = strconv.Itoa(int(attr.AttributeValue.(int32)))
			case kmip.AttributeNameObjectType:
				otype = attr.AttributeValue.(kmip.ObjectType)
			case kmip.AttributeNameState:
				state = attr.AttributeValue.(kmip.State)
			case kmip.AttributeNameName:
				if attr.AttributeIndex != nil && *attr.AttributeIndex != 0 {
					continue
				}
				name = attr.AttributeValue.(kmip.Name).NameValue
			case kmip.AttributeNameCryptographicAlgorithm:
				alg = ttlv.EnumStr(attr.AttributeValue.(kmip.CryptographicAlgorithm))
			case kmip.AttributeNameInitialDate:
				d := time.Since(attr.AttributeValue.(time.Time))
				if d >= 24*time.Hour {
					age = strconv.Itoa(int(d/(24*time.Hour))) + "d"
				} else if d > time.Hour {
					age = strconv.Itoa(int(d/time.Hour)) + "h" + strconv.Itoa(int((d%time.Hour)/time.Minute)) + "m"
				} else if d > time.Minute {
					age = strconv.Itoa(int(d/time.Minute)) + "m"
				} else {
					age = "<1m"
				}
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
		newCell := func(txt string) *tview.TableCell {
			return tview.NewTableCell(txt).SetStyle(style).SetSelectedStyle(style.Reverse(true)).SetExpansion(1)
		}
		mtb.Table.SetCell(i+1, 0, newCell(v.UniqueIdentifier))
		mtb.Table.SetCell(i+1, 1, newCell(ttlv.EnumStr(otype)))
		mtb.Table.SetCell(i+1, 2, newCell(name))
		mtb.Table.SetCell(i+1, 3, newCell(alg))
		mtb.Table.SetCell(i+1, 4, newCell(size))
		mtb.Table.SetCell(i+1, 5, newCell(ttlv.EnumStr(state)))
		mtb.Table.SetCell(i+1, 6, newCell(age))
	}
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
	for i := mtb.Table.GetRowCount() - 1; i > 0; i-- {
		mtb.Table.RemoveRow(i)
	}
	mtb.updateTitle()
}

func (mtb *MobTable) SetObjects(objects []*payloads.GetAttributesResponsePayload) {
	mtb.objects = objects
	mtb.rebuildTable()
}

func (mtb *MobTable) RemoveObject(id string) {
	i := slices.IndexFunc(mtb.objects, func(o *payloads.GetAttributesResponsePayload) bool { return o.UniqueIdentifier == id })
	mtb.objects = slices.Delete(mtb.objects, i, i+1)
	mtb.rebuildTable()
}

func (mtb *MobTable) UpdateObject(object *payloads.GetAttributesResponsePayload) {
	i := slices.IndexFunc(mtb.objects, func(o *payloads.GetAttributesResponsePayload) bool {
		return o.UniqueIdentifier == object.UniqueIdentifier
	})
	if i < 0 {
		return
	}
	mtb.objects[i] = object
	mtb.rebuildTable()
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

func (tb *MobTable) GetSelection() *payloads.GetAttributesResponsePayload {
	row, _ := tb.Table.GetSelection()
	if row > 0 && row <= len(tb.objects) {
		return tb.objects[row-1]
	}
	return nil
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
