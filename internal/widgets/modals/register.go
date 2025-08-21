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
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/phsym/kmip-explorer/internal/components"

	"github.com/ovh/kmip-go"
	"github.com/ovh/kmip-go/kmipclient"
	"github.com/ovh/kmip-go/payloads"

	"github.com/rivo/tview"
)

type Register struct {
	*tview.Flex
	innerFlex    *tview.Flex
	form         *components.Form
	onRegisterCb func(func(*kmipclient.Client) (*payloads.RegisterResponsePayload, error))
	onCancel     func()
}

func (wg *Register) removeItem(label string) {
	idx := wg.form.GetFormItemIndex(label)
	if idx < 0 {
		return
	}
	wg.form.RemoveFormItem(idx)
}

func NewRegister() *Register {
	wg := &Register{form: components.NewForm()}
	wg.form.
		AddInputField("Name", "", 0, nil, nil).
		AddDropDown("Object Type", nil, 0, nil).
		AddButton("OK", wg.done).
		AddButton("Cancel", wg.cancel).
		SetButtonsAlign(tview.AlignCenter).
		SetCancelFunc(wg.cancel)

	wg.form.GetButton(0).SetDisabled(true)

	wg.form.GetFormItemByLabel("Object Type").(*tview.DropDown).SetOptions([]string{"Secret", "X509 Certificate", "AES Key", "Private Key", "Public Key"}, func(option string, optionIndex int) {
		wg.removeItem("Secret Value") //FIXME: This will reset the field even if seletion has not changed
		wg.removeItem("Base64")
		wg.removeItem("PEM")
		wg.removeItem("PEM Key")
		wg.removeItem("Key")
		wg.removeItem("Format")
		wg.Flex.ResizeItem(wg.innerFlex, 9, 0)
		switch option {
		case "Secret":
			wg.form.AddTextArea("Secret Value", "", 0, 5, 0, nil)
			wg.form.AddCheckbox("Base64", false, nil)
			wg.Flex.ResizeItem(wg.innerFlex, 17, 0)
		case "X509 Certificate":
			wg.form.AddTextArea("PEM", "", 65, 5, 0, nil)
			wg.Flex.ResizeItem(wg.innerFlex, 15, 0)
		case "AES Key":
			wg.form.AddTextArea("Key", "", 65, 5, 0, nil)
			wg.form.AddDropDown("Format", []string{"Hex", "Base 64"}, 0, nil)
			wg.Flex.ResizeItem(wg.innerFlex, 17, 0)
		case "Private Key", "Public Key":
			wg.form.AddTextArea("PEM Key", "", 65, 5, 0, nil)
			wg.Flex.ResizeItem(wg.innerFlex, 15, 0)
		case "":
		default:
			panic("Unknown option " + option)
		}
		wg.form.GetButton(0).SetDisabled(false) //TODO: Enable button only is a value is provided
	})

	wg.form.Box.SetBorder(true).SetTitle("Register object")

	wg.innerFlex = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false).
		AddItem(wg.form, 0, 2, true).
		AddItem(nil, 0, 1, false)

	wg.Flex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(wg.innerFlex, 9, 0, true).
		AddItem(nil, 0, 1, false)
	return wg
}

func (wg *Register) reset() {
	wg.form.GetFormItemByLabel("Object Type").(*tview.DropDown).SetCurrentOption(-1)
	wg.form.GetFormItemByLabel("Name").(*tview.InputField).SetText("")
	wg.form.GetButton(0).SetDisabled(true)
	wg.form.SetFocus(0)
}

func (wg *Register) OnCancel(f func()) *Register {
	wg.onCancel = f
	return wg
}

func (wg *Register) OnDone(cb func(func(*kmipclient.Client) (*payloads.RegisterResponsePayload, error))) *Register {
	wg.onRegisterCb = cb
	return wg
}

func (wg *Register) cancel() {
	defer wg.reset()
	if wg.onCancel != nil {
		wg.onCancel()
	}
}

func (wg *Register) done() {
	defer wg.reset()
	if wg.onRegisterCb == nil {
		return
	}
	var f func(client *kmipclient.Client) (*payloads.RegisterResponsePayload, error)

	_, objectType := wg.form.GetFormItemByLabel("Object Type").(*tview.DropDown).GetCurrentOption()
	name := wg.form.GetFormItemByLabel("Name").(*tview.InputField).GetText()

	switch objectType {
	case "Secret":
		secretValueStr := wg.form.GetFormItemByLabel("Secret Value").(*tview.TextArea).GetText()
		b64 := wg.form.GetFormItemByLabel("Base64").(*tview.Checkbox).IsChecked()
		f = func(client *kmipclient.Client) (*payloads.RegisterResponsePayload, error) {
			secretValue := []byte(secretValueStr)
			if b64 {
				var err error
				secretValue, err = base64.StdEncoding.DecodeString(secretValueStr)
				if err != nil {
					return nil, fmt.Errorf("Invalid secret value: %w", err)
				}
			}
			req := client.Register().Secret(kmip.SecretDataTypePassword, secretValue)
			if name != "" {
				req = req.WithName(name)
			}
			return req.Exec()
		}
	case "X509 Certificate":
		pemValue := wg.form.GetFormItemByLabel("PEM").(*tview.TextArea).GetText()
		f = func(client *kmipclient.Client) (*payloads.RegisterResponsePayload, error) {
			req := client.Register().PemCertificate([]byte(pemValue))
			if name != "" {
				req = req.WithName(name)
			}
			return req.Exec()
		}
	case "AES Key":
		data := wg.form.GetFormItemByLabel("Key").(*tview.TextArea).GetText()
		_, format := wg.form.GetFormItemByLabel("Format").(*tview.DropDown).GetCurrentOption()
		f = func(client *kmipclient.Client) (*payloads.RegisterResponsePayload, error) {
			var decode func(string) ([]byte, error)
			switch format {
			case "Hex":
				decode = hex.DecodeString
			case "Base 64":
				decode = base64.StdEncoding.DecodeString
			}
			key, err := decode(data)
			if err != nil {
				return nil, err
			}
			req := client.Register().SymmetricKey(
				kmip.CryptographicAlgorithmAES,
				kmip.CryptographicUsageEncrypt|kmip.CryptographicUsageDecrypt|kmip.CryptographicUsageWrapKey|kmip.CryptographicUsageUnwrapKey,
				key,
			)
			if name != "" {
				req = req.WithName(name)
			}
			return req.Exec()
		}
	case "Private Key":
		pemValue := wg.form.GetFormItemByLabel("PEM Key").(*tview.TextArea).GetText()
		f = func(client *kmipclient.Client) (*payloads.RegisterResponsePayload, error) {
			req := client.Register().PemPrivateKey([]byte(pemValue), kmip.CryptographicUsageSign)
			if name != "" {
				req = req.WithName(name)
			}
			return req.Exec()
		}
	case "Public Key":
		pemValue := wg.form.GetFormItemByLabel("PEM Key").(*tview.TextArea).GetText()
		f = func(client *kmipclient.Client) (*payloads.RegisterResponsePayload, error) {
			req := client.Register().PemPublicKey([]byte(pemValue), kmip.CryptographicUsageVerify)
			if name != "" {
				req = req.WithName(name)
			}
			return req.Exec()
		}
	}

	wg.onRegisterCb(f)
}
