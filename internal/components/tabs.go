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

type Tabs struct {
	*tview.Flex
	choices         []string
	selected        int
	changedCallback func(int, string)
}

var _ tview.Primitive = (*Tabs)(nil)

func NewTabs(choices ...string) *Tabs {
	tabs := &Tabs{
		Flex:    tview.NewFlex().SetDirection(tview.FlexColumn),
		choices: choices,
	}

	for i, c := range choices {
		txt := tview.NewTextView().SetText(c)
		if i == 0 {
			txt.SetTextStyle(tcell.StyleDefault.Reverse(true))
		}
		if i > 0 {
			tabs.Flex.AddItem(tview.NewTextView().SetText(" | "), 3, 0, false)
		}
		txt.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
			if action == tview.MouseLeftClick && txt.InRect(event.Position()) {
				tabs.Select(i)
				return tview.MouseConsumed, nil
			}
			return action, event
		})
		tabs.Flex.AddItem(txt, tview.TaggedStringWidth(c), 0, false)
	}

	return tabs
}

func (t *Tabs) Next() {
	t.Select(t.selected + 1)
}

func (t *Tabs) Prev() {
	t.Select(t.selected - 1)
}

func (t *Tabs) Select(tab int) {
	if tab < 0 {
		tab = len(t.choices) - 1
	} else {
		tab %= len(t.choices)
	}
	if tab == t.selected {
		return
	}
	t.selected, tab = tab, t.selected
	t.changed(tab)
}

func (t *Tabs) GetSelected() (int, string) {
	return t.selected, t.choices[t.selected]
}

func (t *Tabs) SetChangedFunc(f func(int, string)) *Tabs {
	t.changedCallback = f
	return t
}

func (t *Tabs) changed(previous int) {
	t.Flex.GetItem(t.selected * 2).(*tview.TextView).SetTextStyle(tcell.StyleDefault.Reverse(true))
	t.Flex.GetItem(previous * 2).(*tview.TextView).SetTextStyle(tcell.StyleDefault)
	if t.changedCallback == nil {
		return
	}
	t.changedCallback(t.GetSelected())
}

func (t *Tabs) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return t.WrapInputHandler(func(ek *tcell.EventKey, f func(p tview.Primitive)) {
		if ek.Key() == tcell.KeyTAB {
			t.Next()
		} else if ek.Key() == tcell.KeyBacktab {
			t.Prev()
		}
	})
}
