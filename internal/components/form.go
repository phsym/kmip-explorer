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

package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Form struct {
	*tview.Form
}

func NewForm() *Form {
	return &Form{
		tview.NewForm(),
	}
}

func (wg *Form) Draw(screen tcell.Screen) {
	wg.Form.Draw(screen)
	maxWidth := 0
	for ix := range wg.Form.GetFormItemCount() {
		label := wg.Form.GetFormItem(ix).GetLabel()
		if width := tview.TaggedStringWidth(label); width > maxWidth {
			maxWidth = width
		}
	}
	maxWidth++
	for ix := range wg.Form.GetFormItemCount() {
		itm := wg.Form.GetFormItem(ix)
		if itm.HasFocus() {
			itm.SetFormAttributes(maxWidth, tview.Styles.SecondaryTextColor, wg.Box.GetBackgroundColor(), tview.Styles.PrimaryTextColor, tview.Styles.MoreContrastBackgroundColor)
			defer itm.Draw(screen)
		} else {
			itm.SetFormAttributes(maxWidth, tview.Styles.SecondaryTextColor, wg.Box.GetBackgroundColor(), tview.Styles.PrimaryTextColor, tview.Styles.ContrastBackgroundColor)
			itm.Draw(screen)
		}
	}
}
