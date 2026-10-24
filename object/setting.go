// Copyright 2026 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package object

import (
	"fmt"

	"github.com/casbin/caswaf/util"
	"github.com/xorm-io/core"
)

const (
	settingOwner = "admin"
	settingName  = "setting-built-in"
)

type Setting struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime string `xorm:"varchar(100)" json:"updatedTime"`

	// 0 keeps the records forever
	RecordRetentionDays int `json:"recordRetentionDays"`
}

func GetSetting() (*Setting, error) {
	setting := Setting{Owner: settingOwner, Name: settingName}
	existed, err := ormer.Engine.Get(&setting)
	if err != nil {
		return nil, err
	}
	if existed {
		return &setting, nil
	}

	currentTime := util.GetCurrentTime()
	setting.CreatedTime = currentTime
	setting.UpdatedTime = currentTime
	_, err = ormer.Engine.Insert(&setting)
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

func UpdateSetting(setting *Setting) (bool, error) {
	if setting.RecordRetentionDays < 0 {
		return false, fmt.Errorf("the record retention days cannot be negative")
	}

	oldSetting, err := GetSetting()
	if err != nil {
		return false, err
	}

	setting.Owner = settingOwner
	setting.Name = settingName
	setting.CreatedTime = oldSetting.CreatedTime
	setting.UpdatedTime = util.GetCurrentTime()

	affected, err := ormer.Engine.ID(core.PK{settingOwner, settingName}).AllCols().Update(setting)
	if err != nil {
		return false, err
	}

	return affected != 0, nil
}
