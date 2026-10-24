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

package controllers

import (
	"encoding/json"

	"github.com/casbin/caswaf/object"
)

func (c *ApiController) GetSetting() {
	if c.RequireSignedIn() {
		return
	}

	setting, err := object.GetSetting()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(setting)
}

func (c *ApiController) UpdateSetting() {
	if c.RequireSignedIn() {
		return
	}

	var setting object.Setting
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &setting)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.Data["json"] = wrapActionResponse(object.UpdateSetting(&setting))
	c.ServeJSON()
}
