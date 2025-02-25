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

package modals

import (
	"strconv"
	"time"

	"github.com/phsym/kmip-explorer/internal/components"

	"github.com/ovh/kmip-go/kmipclient"
	"github.com/ovh/kmip-go/payloads"

	"github.com/rivo/tview"
)

type Rekey struct {
	*tview.Flex
	form     *components.Form
	onCancel func()
	onDone   func(func(*kmipclient.Client, string) (*payloads.RekeyResponsePayload, error))
}

func NewRekey() *Rekey {
	md := &Rekey{}
	md.form = components.NewForm()
	md.form.AddInputField("Offset days", "", 0, func(textToCheck string, lastChar rune) bool {
		if textToCheck == "" {
			return true
		}
		i, err := strconv.Atoi(textToCheck)
		return err == nil && i >= 0
	}, nil).
		AddButton("OK", md.done).
		AddButton("Cancel", md.cancel).
		SetCancelFunc(md.cancel).
		SetButtonsAlign(tview.AlignCenter)
	md.form.Box.SetBorder(true).SetTitle("Rekey")

	md.Flex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(md.form, 0, 2, true).
			AddItem(nil, 0, 1, false),
			9, 0, true).
		AddItem(nil, 0, 1, false)
	return md
}

func (md *Rekey) OnCancel(cb func()) *Rekey {
	md.onCancel = cb
	return md
}

func (md *Rekey) OnDone(cb func(func(*kmipclient.Client, string) (*payloads.RekeyResponsePayload, error))) *Rekey {
	md.onDone = cb
	return md
}

func (md *Rekey) reset() {
	md.form.SetFocus(0)
	md.form.GetFormItemByLabel("Offset days").(*tview.InputField).SetText("")
}

func (md *Rekey) done() {
	defer md.reset()
	if md.onDone == nil {
		return
	}
	offsetString := md.form.GetFormItemByLabel("Offset days").(*tview.InputField).GetText()
	offset := time.Duration(-1)
	if offsetString != "" {
		i, err := strconv.Atoi(offsetString)
		if err != nil {
			// The input is validated in the form, so this should not happen
			panic(err)
		}
		offset = time.Duration(i) * time.Hour * 24
	}

	md.onDone(func(c *kmipclient.Client, id string) (*payloads.RekeyResponsePayload, error) {
		req := c.Rekey(id)
		if offset >= 0 {
			req = req.WithOffset(offset)
		}
		return req.Exec()
	})
}

func (md *Rekey) cancel() {
	defer md.reset()
	if md.onCancel != nil {
		md.onCancel()
	}
}
