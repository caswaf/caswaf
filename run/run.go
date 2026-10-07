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
	if strings.HasPrefix(siteName, "opendata_") && siteName != "opendata_my" {
		return true
	}
	if strings.HasPrefix(siteName, "openct_") && siteName != "openct_my" {
		return true
	}
	return strings.HasPrefix(siteName, "casos_") && siteName != "casos_my"
}

func wrapRepoError(function string, path string, err error) (int, error) {
	return 0, fmt.Errorf("%s(): path = %s, %s", function, path, err.Error())
}

func CreateRepo(siteName string, needStart bool, diff string, providerName string, orgName string, initAdminPassword string, shouldUpgrade bool, useBinary bool) (int, error) {
	path := GetRepoPath(siteName)
	if !util.FileExist(path) {
		// For new repositories, always allow creation regardless of upgrade mode
		var err error
		if useBinary && isThinSiteCandidate(siteName) && hasSharedCode(siteName) {
			err = createThinSite(siteName)
			if err != nil {
				return wrapRepoError("createThinSite", path, err)
			}
		} else {
			originalName := getOriginalName(siteName)
			repoUrl := getRepoUrl(originalName)
			err = gitClone(repoUrl, path)
			if err != nil {
				return wrapRepoError("gitClone", path, err)
			}
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
			updateAppConfFile(siteName, index, orgName, initAdminPassword)
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
			err = webBuild(siteName, path)
			if err != nil {
				return wrapRepoError("gitWebBuild", path, err)
			}

			err = gitUploadCdn(providerName, siteName)
			if err != nil {
				return wrapRepoError("gitUploadCdn", path, err)
			}
		}

		if shouldRunBinary(siteName, useBinary) {
			_, err = ensureBinary(siteName)
			if err != nil {
				return wrapRepoError("ensureBinary", path, err)
			}

			err = switchBatFile(siteName, true)
			if err != nil {
				return wrapRepoError("switchBatFile", path, err)
			}
		} else {
			_, err = updateBatFile(siteName)
			if err != nil {
				return wrapRepoError("updateBatFile", path, err)
			}
		}

		ensureShortcutFile(siteName)
	} else {
		// For existing repositories, check if upgrade is allowed
		affected := false
		err := checkSharedCode(siteName)
		if err != nil {
			return wrapRepoError("checkSharedCode", path, err)
		}

		if shouldUpgrade && !IsThinSite(siteName) {
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
			err = webBuild(siteName, path)
			if err != nil {
				return wrapRepoError("gitWebBuild", path, err)
			}

			err = gitUploadCdn(providerName, siteName)
			if err != nil {
				return wrapRepoError("gitUploadCdn", path, err)
			}
		} else if shouldUpgrade && !isFrontendBaseDirEnabledRepo(siteName) {
			refreshed, err2 := refreshWebPackage(siteName, path)
			if err2 != nil {
				fmt.Printf("refreshWebPackage(): [%s] %s\n", path, err2.Error())
			}

			if refreshed {
				err = gitUploadCdn(providerName, siteName)
				if err != nil {
					return wrapRepoError("gitUploadCdn", path, err)
				}
			}
		}

		wantBinary := shouldRunBinary(siteName, useBinary)
		if wantBinary {
			_, err = ensureBinary(siteName)
			if err != nil {
				return wrapRepoError("ensureBinary", path, err)
			}
		}

		var batExisted bool
		batExisted, err = updateBatFile(siteName)
		if err != nil {
			return wrapRepoError("updateBatFile", path, err)
		}

		if (!batExisted || needStart) && isBinaryBat(siteName) != wantBinary {
			err = switchBatFile(siteName, wantBinary)
			if err != nil {
				return wrapRepoError("switchBatFile", path, err)
			}
		}

		ensureShortcutFile(siteName)

		if affected {
			if !needStart {
				pid, err := restartProcess(siteName, useBinary)
				if err != nil {
					return wrapRepoError("restartProcess", path, err)
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
