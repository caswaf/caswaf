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

package run

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/beego/beego"
)

func RemoveRepo(siteName string) error {
	repoPath := GetRepoPath(siteName)
	appDir := beego.AppConfig.String("appDir")
	if getMappedName(siteName) == "" || appDir == "" || filepath.Clean(filepath.Dir(repoPath)) != filepath.Clean(appDir) {
		return fmt.Errorf("RemoveRepo() error, refusing to delete %s for site %s", repoPath, siteName)
	}

	err := os.RemoveAll(getShortcutPath(siteName))
	if err != nil {
		return err
	}

	err = stopProcess(siteName)
	if err != nil {
		pids, err2 := getSiteCmdPids(siteName)
		if err2 != nil || len(pids) > 0 {
			return err
		}
	}

	err = os.RemoveAll(getBatPath(siteName))
	if err != nil {
		return err
	}

	fmt.Printf("RemoveRepo(): deleting %s\n", repoPath)
	return os.RemoveAll(repoPath)
}
