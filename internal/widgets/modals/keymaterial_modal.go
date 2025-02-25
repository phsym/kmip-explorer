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
	"encoding/hex"
	"unicode/utf8"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/ovh/kmip-go"
	"github.com/ovh/kmip-go/ttlv"
	"github.com/rivo/tview"
)

type KeyMaterial struct {
	*tview.Flex
	content   *tview.TextView
	onDone    func()
	obj       kmip.Object
	formatRaw bool
}

func NewKeyMaterial() *KeyMaterial {
	md := &KeyMaterial{}
	md.content = tview.NewTextView().
		SetDoneFunc(func(_ tcell.Key) { md.done() })
	md.content.SetBorder(true).SetTitle("Material")

	md.Flex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(md.content, 0, 2, true).
			AddItem(nil, 0, 1, false),
			0, 10, true).
		AddItem(nil, 0, 1, false)
	return md
}

func (wg *KeyMaterial) SetContent(obj kmip.Object) {
	wg.obj = obj
	wg.updateContent()
}

func (wg *KeyMaterial) updateContent() {
	if wg.obj == nil {
		wg.content.SetText("Loading ...")
		return
	}
	if wg.formatRaw {
		wg.content.SetText(string(ttlv.MarshalText(wg.obj)))
		return
	}

	var content string
	var err error
	switch obj := wg.obj.(type) {
	case *kmip.SecretData:
		var data []byte
		data, err = obj.Data()
		if err != nil {
			break
		}
		if utf8.Valid(data) {
			content = string(data)
		} else {
			content = hex.EncodeToString(data)
		}
	case *kmip.SymmetricKey:
		var data []byte
		data, err = obj.KeyMaterial()
		content = hex.EncodeToString(data)
	case *kmip.Certificate:
		content, err = obj.PemCertificate()
	case *kmip.PrivateKey:
		content, err = obj.Pkcs8Pem()
	case *kmip.PublicKey:
		content, err = obj.PkixPem()
	default:
		content = string(ttlv.MarshalText(obj))
	}
	if err != nil {
		content = "Error:" + err.Error()
	}
	wg.content.SetText(content)
}

func (wg *KeyMaterial) OnDone(cb func()) *KeyMaterial {
	wg.onDone = cb
	return wg
}

func (wg *KeyMaterial) reset() {
	wg.content.SetText("")
	wg.obj = nil
}

func (wg *KeyMaterial) done() {
	defer wg.reset()
	if wg.onDone != nil {
		wg.onDone()
	}
}

func (f *KeyMaterial) Draw(screen tcell.Screen) {
	f.Flex.Draw(screen)
	x, y, w, h := f.content.GetRect()
	_, pw := tview.Print(screen, " <c> Copy ", x+2, y+h-1, w-4, tview.AlignLeft, tcell.ColorDeepSkyBlue)
	tview.Print(screen, " <tab> Switch format ", x+2+pw+4, y+h-1, w-2-(pw+6), tview.AlignLeft, tcell.ColorDeepSkyBlue)
}

func (wg *KeyMaterial) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return wg.content.WrapInputHandler(func(ek *tcell.EventKey, f func(p tview.Primitive)) {
		if ek.Key() == tcell.KeyTab {
			wg.formatRaw = !wg.formatRaw
			wg.updateContent()
			return
		} else if ek.Rune() == 'c' {
			//TODO: Display some error if copying to the clipboard failed
			_ = clipboard.WriteAll(wg.content.GetText(true))
		}
		wg.content.InputHandler()(ek, f)
	})
}
