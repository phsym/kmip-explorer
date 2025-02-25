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
	"github.com/phsym/kmip-explorer/internal/components"

	"github.com/ovh/kmip-go"
	"github.com/ovh/kmip-go/kmipclient"
	"github.com/ovh/kmip-go/payloads"

	"github.com/rivo/tview"
)

type Revoke struct {
	*tview.Flex
	form     *components.Form
	onCancel func()
	onDone   func(func(*kmipclient.Client, string) (*payloads.RevokeResponsePayload, error))
}

func NewRevoke() *Revoke {
	md := &Revoke{}
	md.form = components.NewForm()
	md.form.AddDropDown("Reason", []string{
		"Unspecified",
		"Key Compromise",
		"CA Compromise",
		"Affiliation Change",
		"Superseded",
		"Cessation Of Operation",
		"Privilege Withdraw",
	}, 0, nil).
		AddInputField("Message", "", 0, nil, nil).
		AddButton("OK", md.done).
		AddButton("Cancel", md.cancel).
		SetCancelFunc(md.cancel).
		SetButtonsAlign(tview.AlignCenter)
	md.form.Box.SetBorder(true).SetTitle("Revoke an object")

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

func (md *Revoke) OnCancel(cb func()) *Revoke {
	md.onCancel = cb
	return md
}

func (md *Revoke) OnDone(cb func(func(*kmipclient.Client, string) (*payloads.RevokeResponsePayload, error))) *Revoke {
	md.onDone = cb
	return md
}

func (md *Revoke) reset() {
	md.form.SetFocus(0)
	md.form.GetFormItemByLabel("Message").(*tview.InputField).SetText("")
	md.form.GetFormItemByLabel("Reason").(*tview.DropDown).SetCurrentOption(0)
}

func (md *Revoke) done() {
	defer md.reset()
	if md.onDone == nil {
		return
	}
	reasonCode, _ := md.form.GetFormItemByLabel("Reason").(*tview.DropDown).GetCurrentOption()
	msg := md.form.GetFormItemByLabel("Message").(*tview.InputField).GetText()

	md.onDone(func(c *kmipclient.Client, id string) (*payloads.RevokeResponsePayload, error) {
		return c.Revoke(id).
			WithRevocationReasonCode(kmip.RevocationReasonCode(reasonCode + 1)).
			WithRevocationMessage(msg).
			Exec()
	})
}

func (md *Revoke) cancel() {
	defer md.reset()
	if md.onCancel != nil {
		md.onCancel()
	}
}
