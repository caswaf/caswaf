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
	"encoding/json"
	"fmt"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/beego/beego"
)

var username string
var appMap = map[string]string{}

func init() {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}

	username = usr.Username
	tokens := strings.Split(username, "\\")
	if len(tokens) == 2 {
		username = tokens[1]
	}
}

func InitAppMap() {
	res := beego.AppConfig.String("appMap")
	if res != "" {
		err := json.Unmarshal([]byte(res), &appMap)
		if err != nil {
			panic(err)
		}
	}
}

func getMappedName(name string) string {
	for k, v := range appMap {
		if v != "cc" {
			if name == k {
				return v
			}
		}
	}
	for k, v := range appMap {
		if v == "cc" {
			name = strings.Replace(name, k, v, 1)
			name = strings.Replace(name, "cc_00", "cc_", 1)
			return name
		}
	}
	return name
}

func GetRepoPath(name string) string {
	name = getMappedName(name)
	appDir := beego.AppConfig.String("appDir")
	res := filepath.Join(appDir, name)
	return res
}

func getCodeAppConfPath(name string) string {
	name = getMappedName(name)
	appDir := beego.AppConfig.String("appDir")
	return fmt.Sprintf("%s/%s/conf/app.conf", appDir, name)
}

func getBatPath(name string) string {
	if !isWindows() {
		return getUnitPath(name)
	}

	name = getMappedName(name)
	return fmt.Sprintf("C:/Users/%s/Desktop/run/%s.bat", username, name)
}

func getShortcutPath(name string) string {
	if !isWindows() {
		return getUnitWantsPath(name)
	}

	name = getMappedName(name)
	return fmt.Sprintf("C:/Users/%s/AppData/Roaming/Microsoft/Windows/Start Menu/Programs/Startup/%s.bat - %s.lnk", username, name, getShortcut())
}
