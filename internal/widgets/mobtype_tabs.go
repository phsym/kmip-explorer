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
	"github.com/phsym/kmip-explorer/internal/components"

	"github.com/ovh/kmip-go"
)

type MobTypeTabs struct {
	*components.Tabs
	onChange func(kmip.ObjectType, string)
}

func NewMobTypeTabs() *MobTypeTabs {
	mtt := &MobTypeTabs{}
	mtt.Tabs = components.NewTabs("All", "Symmetric Keys", "Private Keys", "Public Keys", "Secrets", "Certificates", "Opaque", "Templates").
		SetChangedFunc(mtt.changed)

	return mtt
}

func (mtt *MobTypeTabs) Current() string {
	_, selected := mtt.Tabs.GetSelected()
	_, name := mtt.intoType(selected)
	return name
}

func (mtt *MobTypeTabs) changed(_ int, selected string) {
	if mtt.onChange == nil {
		return
	}
	mtt.onChange(mtt.intoType(selected))
}

func (mtt *MobTypeTabs) OnChange(cb func(kmip.ObjectType, string)) *MobTypeTabs {
	mtt.onChange = cb
	return mtt
}

func (mtt *MobTypeTabs) intoType(selected string) (kmip.ObjectType, string) {
	switch selected {
	case "All":
		return 0, "All Objects"
	case "Symmetric Keys":
		return kmip.ObjectTypeSymmetricKey, "Symmetric Keys"
	case "Private Keys":
		return kmip.ObjectTypePrivateKey, "Private Keys"
	case "Public Keys":
		return kmip.ObjectTypePublicKey, "Public Keys"
	case "Secrets":
		return kmip.ObjectTypeSecretData, "Secrets"
	case "Certificates":
		return kmip.ObjectTypeCertificate, "Certificates"
	case "Opaque":
		return kmip.ObjectTypeOpaqueObject, "Opaque Objects"
	case "Templates":
		//nolint:staticcheck // We still want to display templates
		return kmip.ObjectTypeTemplate, "Templates"
	default:
		return 0, "All Objects"
	}
}
