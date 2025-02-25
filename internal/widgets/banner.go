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
	"github.com/ovh/kmip-go/kmipclient"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Logo struct {
	*tview.TextView
}

func NewLogo() *Logo {
	tv := tview.NewTextView().SetText(` _  _ __  __ ___ ____  
| |/ |  \/  |_ _|  _ \ 
| ' /| |\/| || || |_) |
| . \| |  | || ||  __/ 
|_|\_|_|  |_|___|_|    `).
		SetTextAlign(tview.AlignRight).
		SetTextColor(tcell.ColorOrange).
		SetWrap(false)
	return &Logo{tv}
}

func (l *Logo) Width() int {
	return 23
}

type Info struct {
	*tview.Table
}

func NewInfo(server, kmipVersion, clientVersion string) *Info {
	infoStyle := tcell.StyleDefault.Bold(true).Foreground(tcell.ColorOrange)
	info := tview.NewTable().
		SetCell(0, 0, tview.NewTableCell("Server Name: ").SetStyle(infoStyle)).
		SetCell(0, 1, tview.NewTableCell(server)).
		SetCell(1, 0, tview.NewTableCell("Client Version: ").SetStyle(infoStyle)).
		SetCell(1, 1, tview.NewTableCell(clientVersion)).
		SetCell(2, 0, tview.NewTableCell("KMIP Version: ").SetStyle(infoStyle)).
		SetCell(2, 1, tview.NewTableCell(kmipVersion))
	return &Info{info}
}

func (inf *Info) UpdateKmipVersion(vers string) {
	inf.SetCell(2, 1, tview.NewTableCell(vers))
}

func (inf *Info) UpdateServerName(name string) {
	inf.SetCell(0, 1, tview.NewTableCell(name))
}

type Help struct {
	*tview.Table
}

func NewHelp() *Help {
	helpStyle := tcell.StyleDefault.Bold(true).Foreground(tcell.ColorDeepSkyBlue)
	help := tview.NewTable().SetSeparator('\t').
		// 1st column
		SetCell(0, 0, tview.NewTableCell("<ctrl+r>").SetStyle(helpStyle)).SetCell(0, 1, tview.NewTableCell("Refresh")).
		SetCell(1, 0, tview.NewTableCell("<shift+c>").SetStyle(helpStyle)).SetCell(1, 1, tview.NewTableCell("Create key")).
		SetCell(2, 0, tview.NewTableCell("<shift+r>").SetStyle(helpStyle)).SetCell(2, 1, tview.NewTableCell("Register")).
		SetCell(3, 0, tview.NewTableCell("<space>").SetStyle(helpStyle)).SetCell(3, 1, tview.NewTableCell("Get content")).
		// 2nd column
		SetCell(0, 2, tview.NewTableCell("<a>").SetStyle(helpStyle)).SetCell(0, 3, tview.NewTableCell("Activate")).
		SetCell(1, 2, tview.NewTableCell("<r>").SetStyle(helpStyle)).SetCell(1, 3, tview.NewTableCell("Revoke")).
		SetCell(2, 2, tview.NewTableCell("<ctrl+d>").SetStyle(helpStyle)).SetCell(2, 3, tview.NewTableCell("Destroy")).
		SetCell(3, 2, tview.NewTableCell("<ctrl+t>").SetStyle(helpStyle)).SetCell(3, 3, tview.NewTableCell("Rekey")).
		// 3rd column
		SetCell(0, 4, tview.NewTableCell("<tab>").SetStyle(helpStyle)).SetCell(0, 5, tview.NewTableCell("Next page")).
		SetCell(1, 4, tview.NewTableCell("<shift+tab>").SetStyle(helpStyle)).SetCell(1, 5, tview.NewTableCell("Previous page")).
		SetCell(2, 4, tview.NewTableCell("<enter>").SetStyle(helpStyle)).SetCell(2, 5, tview.NewTableCell("Browse attributes")).
		SetCell(3, 4, tview.NewTableCell("<q>").SetStyle(helpStyle)).SetCell(3, 5, tview.NewTableCell("Quit"))
	return &Help{help}
}

type Banner struct {
	*tview.Flex
	info *Info
	help *Help
	logo *Logo
}

func NewBanner(version string) *Banner {
	info := NewInfo("", "", version)
	help := NewHelp()
	logo := NewLogo()
	banner := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(info, 0, 1, false).
		AddItem(help, 0, 1, false).
		AddItem(logo, logo.Width(), 0, false)
	return &Banner{banner, info, help, logo}
}

func (b *Banner) Height() int {
	return max(b.info.GetRowCount(), b.help.GetRowCount(), b.logo.GetOriginalLineCount()-1) + 1
}

func (b *Banner) SetClientInfo(client *kmipclient.Client) {
	b.info.UpdateKmipVersion("v" + client.Version().String())
	b.info.UpdateServerName(client.Addr())
}
