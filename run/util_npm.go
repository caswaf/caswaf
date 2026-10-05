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
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/casbin/caswaf/util"
)

const (
	webPackageRetryInterval = 10 * time.Minute
	webPackageTimeout       = 5 * time.Minute
)

var (
	webPackageNames      = map[string]string{"casdoor": "casdoor-web"}
	webPackageRegistries = []string{"https://registry.npmjs.org", "https://registry.npmmirror.com"}
	webPackageMissMap    = map[string]time.Time{}
	webPackageMissLock   = &sync.Mutex{}
)

type webPackageJson struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func getWebPackage(siteName string, path string) (string, string) {
	name, ok := webPackageNames[getOriginalName(siteName)]
	if !ok {
		return "", ""
	}

	out, err := runGitCommand(path, "diff", "--name-only", "HEAD", "--", "web")
	if err != nil || strings.TrimSpace(out) != "" {
		return "", ""
	}

	hash, err := gitGetLatestCommitHash(path)
	if err != nil {
		return "", ""
	}
	return name, "0.0.0-c" + hash
}

func isWebPackageInstalled(path string, name string, version string) bool {
	data, err := os.ReadFile(filepath.Join(path, "web/build/package.json"))
	if err != nil {
		return false
	}

	pkg := webPackageJson{}
	err = json.Unmarshal(data, &pkg)
	return err == nil && pkg.Name == name && pkg.Version == version && util.FileExist(filepath.Join(path, "web/build/index.html"))
}

func downloadWebPackage(name string, version string, dir string) (bool, error) {
	client := &http.Client{Timeout: webPackageTimeout}
	var lastErr error
	for _, registry := range webPackageRegistries {
		url := fmt.Sprintf("%s/%s/-/%s-%s.tgz", registry, name, name, version)
		resp, err := client.Get(url)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound {
				lastErr = fmt.Errorf("GET %s: %s", url, resp.Status)
			}
			continue
		}

		err = extractWebPackage(resp.Body, dir)
		resp.Body.Close()
		if err != nil {
			return false, fmt.Errorf("extract %s: %s", url, err.Error())
		}

		fmt.Printf("downloadWebPackage(): [%s] -> [%s]\n", url, dir)
		return true, nil
	}
	return false, lastErr
}

func extractWebPackage(r io.Reader, dir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}

		name := strings.TrimPrefix(header.Name, "package/")
		if !filepath.IsLocal(name) {
			return fmt.Errorf("unexpected file in package: %s", header.Name)
		}

		target := filepath.Join(dir, name)
		err = os.MkdirAll(filepath.Dir(target), os.ModePerm)
		if err != nil {
			return err
		}

		f, err := os.Create(target)
		if err != nil {
			return err
		}
		_, err = io.Copy(f, tr)
		f.Close()
		if err != nil {
			return err
		}
	}

	if !util.FileExist(filepath.Join(dir, "index.html")) {
		return fmt.Errorf("index.html not found in package")
	}
	return nil
}

func renameWithRetry(oldPath string, newPath string) error {
	var err error
	for i := 0; i < 10; i++ {
		err = os.Rename(oldPath, newPath)
		if err == nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return err
}

func installWebPackage(siteName string, path string) (bool, error) {
	name, version := getWebPackage(siteName, path)
	if name == "" {
		return false, nil
	}
	if isWebPackageInstalled(path, name, version) {
		return true, nil
	}

	key := name + "@" + version
	webPackageMissLock.Lock()
	missTime, ok := webPackageMissMap[key]
	webPackageMissLock.Unlock()
	if ok && time.Since(missTime) < webPackageRetryInterval {
		return false, nil
	}

	webDir := filepath.Join(path, "web")
	tmpDir := filepath.Join(webDir, "build-npm")
	_ = os.RemoveAll(tmpDir)

	found, err := downloadWebPackage(name, version, tmpDir)
	if err != nil || !found {
		_ = os.RemoveAll(tmpDir)
		webPackageMissLock.Lock()
		webPackageMissMap[key] = time.Now()
		webPackageMissLock.Unlock()
		return false, err
	}

	buildDir := filepath.Join(webDir, "build")
	oldDir := filepath.Join(webDir, "build-npm-old")
	_ = os.RemoveAll(oldDir)
	if util.FileExist(buildDir) {
		err = renameWithRetry(buildDir, oldDir)
		if err != nil {
			_ = os.RemoveAll(tmpDir)
			return false, err
		}
	}

	err = renameWithRetry(tmpDir, buildDir)
	if err != nil {
		_ = renameWithRetry(oldDir, buildDir)
		return false, err
	}

	_ = os.RemoveAll(oldDir)
	return true, nil
}

func webBuild(siteName string, path string) error {
	installed, err := installWebPackage(siteName, path)
	if installed {
		return nil
	}
	if err != nil {
		fmt.Printf("installWebPackage(): [%s] %s, building it instead\n", path, err.Error())
	}

	return gitWebBuild(path)
}

func refreshWebPackage(siteName string, path string) (bool, error) {
	name, version := getWebPackage(siteName, path)
	if name == "" || isWebPackageInstalled(path, name, version) {
		return false, nil
	}

	return installWebPackage(siteName, path)
}
