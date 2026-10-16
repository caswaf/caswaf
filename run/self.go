// Copyright 2023 The casbin Authors. All Rights Reserved.
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

package run

import (
	"runtime"
	"strings"

	"github.com/beego/beego"
	"github.com/casbin/caswaf/util"
)

func InitSelfStart() {
	if runtime.GOOS != "windows" && !isSystemdService() {
		return
	}

	hostname := util.GetHostname()
	if strings.Count(hostname, "-") == 2 {
		return
	}

	siteName := beego.AppConfig.String("dbName")
	if siteName == "" {
		panic("InitSelfStart() error, siteName should not be empty")
	}

	_, err := updateBatFile(siteName)
	if err != nil {
		panic(err)
	}

	shortcutPath := getShortcutPath(siteName)
	if !util.FileExist(shortcutPath) {
		err = updateShortcutFile(siteName)
		if err != nil {
			panic(err)
		}
	}
}
