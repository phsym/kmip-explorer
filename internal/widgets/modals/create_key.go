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

	"github.com/phsym/kmip-explorer/internal/components"

	"github.com/ovh/kmip-go"
	"github.com/ovh/kmip-go/kmipclient"

	"github.com/rivo/tview"
)

type CreateKey struct {
	*tview.Flex
	innerFlex *tview.Flex
	form      *components.Form
	onCancel  func()
	onDone    func(func(*kmipclient.Client) (kmip.OperationPayload, error))
}

func (wg *CreateKey) removeItem(label string) {
	idx := wg.form.GetFormItemIndex(label)
	if idx < 0 {
		return
	}
	wg.form.RemoveFormItem(idx)
}

func NewCreateKey() *CreateKey {
	wg := &CreateKey{form: components.NewForm()}
	wg.form.
		AddInputField("Name", "", 0, nil, nil).
		// AddCheckbox("Sensitive", false, nil).
		// AddCheckbox("Extractable", false, nil).
		AddDropDown("Key Type", nil, 0, nil).
		AddButton("OK", wg.done).
		AddButton("Cancel", wg.cancel).
		SetCancelFunc(wg.cancel).
		SetButtonsAlign(tview.AlignCenter)

	wg.form.GetButton(0).SetDisabled(true)

	wg.form.GetFormItemByLabel("Key Type").(*tview.DropDown).SetOptions([]string{"AES", "RSA", "EC"}, func(option string, optionIndex int) {
		wg.removeItem("Key Size")
		wg.removeItem("Modulus Size")
		wg.removeItem("Curve Type")
		// wg.removeItem("Encryption")
		// wg.removeItem("Key Wrapping")
		// wg.removeItem("Signature")
		// wg.Flex.ResizeItem(wg.innerFlex, 15, 0)
		wg.Flex.ResizeItem(wg.innerFlex, 9, 0)
		switch option {
		case "AES":
			wg.form.AddDropDown("Key Size", []string{"128", "192", "256"}, 0, nil)
			// wg.form.AddCheckbox("Encryption", true, nil)
			// wg.form.AddCheckbox("Key Wrapping", false, nil)
			// wg.Flex.ResizeItem(wg.innerFlex, 19, 0)
			wg.Flex.ResizeItem(wg.innerFlex, 11, 0)
		case "RSA":
			wg.form.AddDropDown("Modulus Size", []string{"2048", "3072", "4096"}, 0, nil)
			// wg.form.AddCheckbox("Signature", true, nil)
			// wg.form.AddCheckbox("Encryption", false, nil)
			// wg.Flex.ResizeItem(wg.innerFlex, 19, 0)
			wg.Flex.ResizeItem(wg.innerFlex, 11, 0)
		case "EC":
			wg.form.AddDropDown("Curve Type", []string{"P-256", "P-384", "P-521"}, 0, nil)
			// wg.form.AddCheckbox("Signature", true, nil)
			// wg.Flex.ResizeItem(wg.innerFlex, 17, 0)
			wg.Flex.ResizeItem(wg.innerFlex, 11, 0)
		case "":
		default:
			panic("Unknown option " + option)
		}
		wg.form.GetButton(0).SetDisabled(false)
	})

	wg.form.Box.SetBorder(true).SetTitle("Create object")

	wg.innerFlex = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false).
		AddItem(wg.form, 0, 2, true).
		AddItem(nil, 0, 1, false)

	wg.Flex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		// AddItem(wg.innerFlex, 15, 0, true).
		AddItem(wg.innerFlex, 9, 0, true).
		AddItem(nil, 0, 1, false)
	return wg
}

func (wg *CreateKey) SetCancelFunc(f func()) *CreateKey {
	wg.onCancel = f
	return wg
}

func (wg *CreateKey) reset() {
	wg.form.GetFormItemByLabel("Key Type").(*tview.DropDown).SetCurrentOption(-1)
	wg.form.GetFormItemByLabel("Name").(*tview.InputField).SetText("")
	// wg.form.GetFormItemByLabel("Sensitive").(*tview.Checkbox).SetChecked(false)
	// wg.form.GetFormItemByLabel("Extractable").(*tview.Checkbox).SetChecked(false)

	wg.form.GetButton(0).SetDisabled(true)
	wg.form.SetFocus(0)
}

func (wg *CreateKey) cancel() {
	defer wg.reset()
	if wg.onCancel != nil {
		wg.onCancel()
	}
}

func (wg *CreateKey) done() {
	defer wg.reset()
	if wg.onDone == nil {
		return
	}
	var f func(c *kmipclient.Client) (kmip.OperationPayload, error)

	_, kty := wg.form.GetFormItemByLabel("Key Type").(*tview.DropDown).GetCurrentOption()
	name := wg.form.GetFormItemByLabel("Name").(*tview.InputField).GetText()
	switch kty {
	case "AES":
		_, ksize := wg.form.GetFormItemByLabel("Key Size").(*tview.DropDown).GetCurrentOption()
		size, err := strconv.Atoi(ksize)
		if err != nil {
			panic("Invalid AES key size:" + err.Error())
		}
		f = func(c *kmipclient.Client) (kmip.OperationPayload, error) {
			req := c.Create().AES(size, kmip.Encrypt|kmip.Decrypt|kmip.WrapKey|kmip.UnwrapKey)
			if name != "" {
				req = req.WithName(name)
			}
			return req.Exec()
		}
	case "RSA":
		_, ksize := wg.form.GetFormItemByLabel("Modulus Size").(*tview.DropDown).GetCurrentOption()
		size, err := strconv.Atoi(ksize)
		if err != nil {
			panic("Invalid rsa modulus size: " + err.Error())
		}
		f = func(c *kmipclient.Client) (kmip.OperationPayload, error) {
			req := c.CreateKeyPair().RSA(size, kmip.Sign, kmip.Verify)
			if name != "" {
				req = req.PublicKey().WithName(name + "-Public").
					PrivateKey().WithName(name + "-Private")
			}
			return req.Exec()
		}
	case "EC":
		_, crv := wg.form.GetFormItemByLabel("Curve Type").(*tview.DropDown).GetCurrentOption()
		var curve kmip.RecommendedCurve
		switch crv {
		case "P-256":
			curve = kmip.P_256
		case "P-384":
			curve = kmip.P_384
		case "P-521":
			curve = kmip.P_521
		default:
			panic("Invalid EC curve " + crv)
		}
		f = func(c *kmipclient.Client) (kmip.OperationPayload, error) {
			req := c.CreateKeyPair().ECDSA(curve, kmip.Sign, kmip.Verify)
			if name != "" {
				req = req.PublicKey().WithName(name + "-Public").
					PrivateKey().WithName(name + "-Private")
			}
			return req.Exec()
		}
	default:
		panic("Unexpected key type " + kty)
	}
	wg.onDone(f)
}

func (wg *CreateKey) SetDoneFunc(f func(func(*kmipclient.Client) (kmip.OperationPayload, error))) *CreateKey {
	wg.onDone = f
	return wg
}
