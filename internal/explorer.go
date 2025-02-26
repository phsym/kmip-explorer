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

package internal

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/ovh/kmip-go"
	"github.com/ovh/kmip-go/kmipclient"
	"github.com/ovh/kmip-go/payloads"
	"github.com/ovh/kmip-go/ttlv"
	"github.com/phsym/kmip-explorer/internal/widgets"
	"github.com/phsym/kmip-explorer/internal/widgets/modals"
	"github.com/rivo/tview"
)

type Explorer struct {
	app        *tview.Application
	search     *tview.InputField
	attributes *tview.TextView
	table      *widgets.MobTable
	tabs       *widgets.MobTypeTabs

	errorModal        *tview.Modal
	confirmModal      *tview.Modal
	revokeModal       *modals.Revoke
	rekeyModal        *modals.Rekey
	createWidget      *modals.CreateKey
	registerWidget    *modals.Register
	keyMaterialWidget *modals.KeyMaterial

	pages *tview.Pages

	contentLayout *tview.Flex

	typeFilter kmip.ObjectType

	client *kmipclient.Client
}

func NewExplorer(client *kmipclient.Client, version, latestVersion string) *Explorer {
	ex := &Explorer{
		client: client,
	}
	ex.app = tview.NewApplication()
	// ex.app.EnableMouse(true)

	ex.search = tview.NewInputField().SetPlaceholder("Input").SetLabel("> ").
		SetLabelColor(tcell.ColorDefault).
		SetFieldBackgroundColor(tcell.ColorNone).
		SetPlaceholderStyle(tcell.StyleDefault.
			Background(tcell.ColorNone).
			Foreground(tcell.ColorDarkGray),
		)
	ex.search.SetBorder(true).SetTitle("Search").SetTitleAlign(tview.AlignLeft)

	ex.attributes = tview.NewTextView().SetDynamicColors(true)
	ex.attributes.SetBorder(true).SetTitle("Attributes")

	ex.table = widgets.NewMobTable().
		OnSelected(func(garp *payloads.GetAttributesResponsePayload) {
			if garp != nil {
				ex.app.SetFocus(ex.attributes)
				ex.contentLayout.ResizeItem(ex.attributes, 0, 6)
			}
		}).
		OnSelectionChanged(func(garp *payloads.GetAttributesResponsePayload) {
			ex.rebuildAttributes(garp)
		}).
		OnContentUpdate(func() {
			ex.rebuildAttributes(ex.table.GetSelection())
		})
	ex.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			ex.tabs.Next()
			return nil
		}
		if event.Key() == tcell.KeyBacktab {
			ex.tabs.Prev()
			return nil
		}
		if event.Rune() == 'C' {
			ex.pages.ShowPage("create")
			ex.app.SetFocus(ex.createWidget)
			return nil
		}
		if event.Rune() == 'R' {
			ex.pages.ShowPage("register")
			ex.app.SetFocus(ex.registerWidget)
			return nil
		}

		obj := ex.table.GetSelection()
		if obj == nil {
			return event
		}
		if event.Rune() == 'r' {
			ex.revokeModal.OnDone(func(f func(*kmipclient.Client, string) (*payloads.RevokeResponsePayload, error)) {
				ex.pages.HidePage("revoke")
				ex.app.SetFocus(ex.table)
				ex.askConfirm("Confirm Revoke", fmt.Sprintf("Revoke object %s ?", obj.UniqueIdentifier), func() {
					go func() {
						resp, err := f(client, obj.UniqueIdentifier)
						if err != nil {
							ex.setError(err)
							return
						}
						ex.update(resp.UniqueIdentifier)
					}()
				})
			})
			ex.pages.ShowPage("revoke")
			ex.app.SetFocus(ex.revokeModal)
			return nil
		} else if event.Rune() == 'a' {
			go ex.activate(obj.UniqueIdentifier)
			return nil
		} else if event.Key() == tcell.KeyCtrlD {
			ex.askConfirm("Confirm Destroy", fmt.Sprintf("Destroy object %s ?", obj.UniqueIdentifier), func() {
				go ex.destroy(obj.UniqueIdentifier)
			})
			return nil
		} else if event.Key() == tcell.KeyCtrlT {
			ex.rekeyModal.OnDone(func(f func(*kmipclient.Client, string) (*payloads.RekeyResponsePayload, error)) {
				ex.pages.HidePage("rekey")
				ex.app.SetFocus(ex.table)
				ex.askConfirm("Confirm Rekeying", fmt.Sprintf("Rekey object %s ?", obj.UniqueIdentifier), func() {
					go func() {
						_, err := f(client, obj.UniqueIdentifier)
						if err != nil {
							ex.setError(err)
							return
						}
						ex.refresh(false)
					}()
				})
			})
			ex.pages.ShowPage("rekey")
			ex.app.SetFocus(ex.rekeyModal)
			return nil
		} else if event.Rune() == ' ' {
			go func() {
				resp, err := ex.client.Get(obj.UniqueIdentifier).Exec()
				if err != nil {
					ex.setError(err)
					return
				}
				ex.app.QueueUpdateDraw(func() {
					ex.pages.ShowPage("key-material")
					ex.app.SetFocus(ex.keyMaterialWidget)
					ex.keyMaterialWidget.SetContent(resp.Object)
				})
			}()
			return nil
		}
		return event
	})

	content := tview.NewFlex().
		AddItem(ex.table, 0, 2, false).
		AddItem(ex.attributes, 0, 0, false)

	banner := widgets.NewBanner(version, latestVersion)
	banner.SetClientInfo(client)

	ex.tabs = widgets.NewMobTypeTabs().
		OnChange(func(ot kmip.ObjectType, s string) {
			ex.table.Clear(true)
			ex.typeFilter = ot
			ex.table.SetTitle(s)
			go ex.refresh(true)
		})
	ex.table.SetTitle(ex.tabs.Current())

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(banner, banner.Height(), 0, false).
		AddItem(ex.tabs, 1, 0, false).
		AddItem(ex.search, 0, 0, false).
		AddItem(content, 0, 1, false)

	ex.contentLayout = content

	ex.errorModal = tview.NewModal().SetBackgroundColor(tcell.ColorDarkRed)
	ex.errorModal.Box.SetBackgroundColor(tcell.ColorDarkRed)
	ex.errorModal.SetBorderColor(tcell.ColorWhite)
	ex.errorModal.SetTitle("Error")
	ex.errorModal.AddButtons([]string{"OK"})
	ex.errorModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		ex.pages.HidePage("error")
		ex.app.SetFocus(ex.table)
	})

	ex.confirmModal = tview.NewModal().AddButtons([]string{"Yes", "No"})

	ex.revokeModal = modals.NewRevoke().
		OnCancel(func() {
			ex.pages.HidePage("revoke")
			ex.app.SetFocus(ex.table)
		})

	ex.rekeyModal = modals.NewRekey().
		OnCancel(func() {
			ex.pages.HidePage("rekey")
			ex.app.SetFocus(ex.table)
		})

	ex.createWidget = modals.NewCreateKey().
		SetCancelFunc(func() {
			ex.pages.HidePage("create")
			ex.app.SetFocus(ex.table)
		}).
		SetDoneFunc(func(f func(*kmipclient.Client) (kmip.OperationPayload, error)) {
			ex.pages.HidePage("create")
			ex.app.SetFocus(ex.table)
			go func() {
				_, err := f(ex.client)
				if err != nil {
					ex.setError(err)
					return
				}
				ex.refresh(false)
			}()
		})
	ex.registerWidget = modals.NewRegister().
		OnCancel(func() {
			ex.pages.HidePage("register")
			ex.app.SetFocus(ex.table)
		}).
		OnDone(func(f func(*kmipclient.Client) (*payloads.RegisterResponsePayload, error)) {
			ex.pages.HidePage("register")
			ex.app.SetFocus(ex.table)
			go func() {
				_, err := f(ex.client)
				if err != nil {
					ex.setError(err)
					return
				}
				ex.refresh(false)
			}()
		})

	ex.keyMaterialWidget = modals.NewKeyMaterial().
		OnDone(func() {
			ex.pages.HidePage("key-material")
			ex.app.SetFocus(ex.table)
		})

	ex.pages = tview.NewPages().
		AddPage("main", layout, true, true).
		AddPage("error", ex.errorModal, true, false).
		AddPage("confirm", ex.confirmModal, true, false).
		AddPage("register", ex.registerWidget, true, false).
		AddPage("revoke", ex.revokeModal, true, false).
		AddPage("rekey", ex.rekeyModal, true, false).
		AddPage("create", ex.createWidget, true, false).
		AddPage("key-material", ex.keyMaterialWidget, true, false)

	ex.app.SetRoot(ex.pages, true).SetFocus(ex.table).SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' && !ex.search.HasFocus() && !ex.revokeModal.HasFocus() && !ex.createWidget.HasFocus() && !ex.registerWidget.HasFocus() && !ex.rekeyModal.HasFocus() {
			//TODO: Move to table input handler ?
			ex.app.Stop()
			return nil
		}
		if event.Rune() == '/' && !ex.search.HasFocus() && !ex.revokeModal.HasFocus() && !ex.createWidget.HasFocus() && !ex.registerWidget.HasFocus() && !ex.rekeyModal.HasFocus() {
			//TODO: Move to table input handler ?
			layout.ResizeItem(ex.search, 3, 0)
			ex.app.SetFocus(ex.search)
			return nil
		}
		if event.Key() == tcell.KeyESC {
			if ex.search.HasFocus() {
				//TODO: Move this handler to the searchbar input handler
				ex.search.SetText("")
				layout.ResizeItem(ex.search, 0, 0)
				ex.app.SetFocus(ex.table)
				return nil
			} else if ex.attributes.HasFocus() {
				//TODO: Move this handler to the attributes input handler
				ex.attributes.ScrollToBeginning()
				ex.app.SetFocus(ex.table)
				ex.contentLayout.ResizeItem(ex.attributes, 0, 1)
				return nil
			}
		}
		if event.Key() == tcell.KeyEnter {
			if ex.search.HasFocus() {
				//TODO: Move this handler to the searchbar input handler
				ex.app.SetFocus(ex.table)
				return nil
			}
		}
		if event.Key() == tcell.KeyCtrlR {
			//TODO: Move to table input handler ?
			go ex.refresh(false)
		}
		return event
	})

	return ex
}

func (ex *Explorer) init() {
	go ex.refresh(true)
}

func (ex *Explorer) Run() error {
	ex.init()
	defer ex.app.Stop()
	return ex.app.Run()
}

func (ex *Explorer) setError(err error) {
	ex.app.QueueUpdateDraw(func() {
		ex.errorModal.SetText(err.Error())
		ex.pages.ShowPage("error")
		ex.app.SetFocus(ex.errorModal)
	})
}

func (ex *Explorer) rebuildAttributes(garp *payloads.GetAttributesResponsePayload) {
	ex.attributes.Clear()
	if garp != nil {
		enc := ttlv.NewTextEncoder()
		strBld := strings.Builder{}
		//FIXME: init the regex in a static var
		attributeValueHdrRegex := regexp.MustCompile(`^AttributeValue \(.+\): `)
		attributeValueFieldsRegex := regexp.MustCompile(`(.+) \(.+\): `)
		for _, attr := range garp.Attribute {
			strBld.WriteString("[green]")
			strBld.WriteString(string(attr.AttributeName))
			strBld.WriteString(": [white]")
			enc.TagAny(kmip.TagAttributeValue, attr.AttributeValue)
			//XXX: It's a bit dirty but it works well for now. The regex could conflict with some values still. We need to find a better way
			value := attributeValueHdrRegex.ReplaceAll(enc.Bytes(), nil)
			value = attributeValueFieldsRegex.ReplaceAll(value, []byte("[yellow]$1: [white]"))
			strBld.Write(value)
			strBld.WriteByte('\n')
			enc.Clear()
		}
		ex.attributes.SetText(strBld.String())
		ex.contentLayout.ResizeItem(ex.attributes, 0, 1)
	} else {
		ex.contentLayout.ResizeItem(ex.attributes, 0, 0)
	}
}

func (ex *Explorer) refresh(resetSelect bool) {
	req := ex.client.Locate()
	if ex.typeFilter != 0 {
		req = req.WithObjectType(ex.typeFilter)
	}
	resp, err := req.Exec()
	if err != nil {
		ex.setError(err)
		return
	}
	attrs := make([]*payloads.GetAttributesResponsePayload, len(resp.UniqueIdentifier))
	for i, obj := range resp.UniqueIdentifier {
		attrs[i], err = ex.client.GetAttributes(obj).Exec()
		if err != nil {
			ex.setError(err)
			return
		}
	}
	ex.app.QueueUpdateDraw(func() {
		if resetSelect {
			ex.table.ScrollToBeginning()
			ex.table.Select(0, 0)
		}
		ex.table.SetObjects(attrs)
	})
}

func (ex *Explorer) update(id string) {
	attrs, err := ex.client.GetAttributes(id).Exec()
	if err != nil {
		ex.setError(err)
		return
	}
	ex.app.QueueUpdateDraw(func() {
		ex.table.UpdateObject(attrs)
	})

}

func (ex *Explorer) askConfirm(title, question string, f func()) {
	ex.confirmModal.SetText(question).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Yes" {
				f()
			}
			ex.pages.HidePage("confirm")
			ex.app.SetFocus(ex.table)
		}).
		SetFocus(1). // Focus on the "No" button
		SetTitle(title)
	ex.pages.ShowPage("confirm")
	ex.app.SetFocus(ex.confirmModal)
}

func (ex *Explorer) activate(id string) {
	if _, err := ex.client.Activate(id).Exec(); err != nil {
		ex.setError(err)
		return
	}
	ex.update(id)
}

func (ex *Explorer) destroy(id string) {
	if _, err := ex.client.Destroy(id).Exec(); err != nil {
		ex.setError(err)
		return
	}
	ex.app.QueueUpdateDraw(func() {
		ex.table.RemoveObject(id)
	})
}
