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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/beego/beego"
	"github.com/casbin/caswaf/util"
)

func isTargetRepo(siteName string) bool {
	return strings.HasPrefix(siteName, "cc_") || strings.HasPrefix(siteName, "casibase_customer_") || strings.Count(siteName, "_") == 2
}

func isFrontendBaseDirEnabledRepo(siteName string) bool {
	if strings.HasPrefix(siteName, "casdoor_") && siteName != "casdoor_my" {
		return true
	}
	if strings.HasPrefix(siteName, "casibase_") && !strings.HasSuffix(siteName, "keli") && !strings.HasSuffix(siteName, "med2") {
		return true
	}
	return strings.HasPrefix(siteName, "opendata_") && siteName != "opendata_my"
}

func wrapRepoError(function string, path string, err error) (int, error) {
	return 0, fmt.Errorf("%s(): path = %s, %s", function, path, err.Error())
}

func CreateRepo(siteName string, needStart bool, diff string, providerName string, orgName string, shouldUpgrade bool) (int, error) {
	path := GetRepoPath(siteName)
	if !util.FileExist(path) {
		// For new repositories, always allow creation regardless of upgrade mode
		originalName := getOriginalName(siteName)
		repoUrl := getRepoUrl(originalName)
		err := gitClone(repoUrl, path)
		if err != nil {
			return wrapRepoError("gitClone", path, err)
		}

		dbInstanceId := beego.AppConfig.String("dbInstanceId")
		if dbInstanceId == "" {
			_, err = gitCreateDatabase(siteName)
			if err != nil {
				return wrapRepoError("gitCreateDatabase", path, err)
			}
		} else {
			_, err = gitCreateDatabaseCloud(siteName)
			if err != nil {
				return wrapRepoError("gitCreateDatabaseCloud", path, err)
			}
		}

		needWebBuild := false
		if isTargetRepo(siteName) {
			index := getNameIndex(siteName)
			updateAppConfFile(siteName, index, orgName)
			if index == 0 {
				needWebBuild = true
			}
		} else {
			needWebBuild = true

			if diff != "" {
				err = gitApply(path, diff)
				if err != nil {
					return wrapRepoError("gitApply", path, err)
				}
			}
		}

		if needWebBuild && !isFrontendBaseDirEnabledRepo(siteName) {
			err = gitWebBuild(path)
			if err != nil {
				return wrapRepoError("gitWebBuild", path, err)
			}

			err = gitUploadCdn(providerName, siteName)
			if err != nil {
				return wrapRepoError("gitUploadCdn", path, err)
			}
		}

		_, err = updateBatFile(siteName)
		if err != nil {
			return wrapRepoError("updateBatFile", path, err)
		}

		shortcutPath := getShortcutPath(siteName)
		if !util.FileExist(shortcutPath) {
			err = updateShortcutFile(siteName)
			if err != nil {
				return wrapRepoError("updateShortcutFile", path, err)
			}
		}
	} else {
		// For existing repositories, check if upgrade is allowed
		affected := false
		var err error
		if shouldUpgrade {
			affected, err = gitPull(path)
			if err != nil {
				return wrapRepoError("gitPull", path, err)
			}
		}

		needWebBuild := false
		if affected {
			if isTargetRepo(siteName) {
				index := getNameIndex(siteName)
				if index == 0 {
					needWebBuild = true
				}
			} else {
				needWebBuild = true
			}
		} else {
			webIndex := filepath.Join(path, "web/build/index.html")
			if !util.FileExist(webIndex) {
				needWebBuild = true
			}

			if isTargetRepo(siteName) {
				index := getNameIndex(siteName)
				if index != 0 {
					needWebBuild = false
				}
			}
		}

		if needWebBuild && !isFrontendBaseDirEnabledRepo(siteName) {
			err = gitWebBuild(path)
			if err != nil {
				return wrapRepoError("gitWebBuild", path, err)
			}

			err = gitUploadCdn(providerName, siteName)
			if err != nil {
				return wrapRepoError("gitUploadCdn", path, err)
			}
		}

		_, err = updateBatFile(siteName)
		if err != nil {
			return wrapRepoError("updateBatFile", path, err)
		}

		shortcutPath := getShortcutPath(siteName)
		if !util.FileExist(shortcutPath) {
			err = updateShortcutFile(siteName)
			if err != nil {
				return wrapRepoError("updateShortcutFile", path, err)
			}
		}

		if affected {
			if !needStart {
				if !strings.HasPrefix(siteName, "casdoor") && !strings.HasPrefix(siteName, "casibase") && !strings.HasPrefix(siteName, "opendata") {
					err = stopProcess(siteName)
					if err != nil {
						return wrapRepoError("stopProcess", path, err)
					}
				}

				err = startProcess(siteName)
				if err != nil {
					return wrapRepoError("startProcess", path, err)
				}

				var pid int
				pid, err = getPid(siteName)
				if err != nil {
					return wrapRepoError("getPid", path, err)
				}

				return pid, nil
			}
		}
	}

	if needStart {
		err := startProcess(siteName)
		if err != nil {
			return wrapRepoError("startProcess", path, err)
		}

		pid, err := getPid(siteName)
		if err != nil {
			return wrapRepoError("getPid", path, err)
		}

		return pid, nil
	}

	return 0, nil
}
