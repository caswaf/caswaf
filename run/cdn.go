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
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/beego/beego"
	"github.com/casbin/caswaf/storage"
	"github.com/casbin/caswaf/util"
)

func filterFiles(filenames []string, folder string, siteName string) []string {
	res := []string{}
	for _, filename := range filenames {
		if !strings.HasSuffix(filename, folder) {
			continue
		}

		if strings.HasPrefix(siteName, "casdoor") {
			if strings.Contains(filename, ".chunk.js") || strings.Contains(filename, ".chunk.css") {
				continue
			}
		}

		res = append(res, filename)
	}
	return res
}

func uploadFolder(provider storage.StorageProvider, buildDir string, relDir string, filenames []string, rootDir string) (string, error) {
	domainUrl := ""

	path := filepath.Join(buildDir, relDir)
	for _, filename := range filenames {
		data, err := os.ReadFile(filepath.Join(path, filename))
		if err != nil {
			return "", err
		}
		fileBuffer := bytes.NewBuffer(data)

		objectKey := strings.ReplaceAll(filepath.Join(relDir, filename), "\\", "/")
		fileUrl, err := provider.PutObject("Built-in-Untracked", "", objectKey, fileBuffer)
		if err != nil {
			return "", err
		}

		index := strings.Index(fileUrl, "/"+rootDir)
		if index == -1 {
			return "", fmt.Errorf("uploadFolder() error, fileUrl should contain \"/%s/\", fileUrl = %s", rootDir, fileUrl)
		}

		domainUrl = fileUrl[:index+len("/"+rootDir)] + "/"
		fmt.Printf("uploadFolder(): [/%s] -> [%s]\n", objectKey, fileUrl)
	}

	return domainUrl, nil
}

func updateHtml(domainUrl string, buildDir string, rootDir string) {
	htmlPath := filepath.Join(buildDir, "index.html")
	html := util.ReadStringFromPath(htmlPath)

	html = strings.Replace(html, fmt.Sprintf("\"/%s/", rootDir), fmt.Sprintf("\"%s", domainUrl), -1)
	util.WriteStringToPath(html, htmlPath)

	fmt.Printf("updateHtml(): index.html content:\n%s\n%s\n%s\n", strings.Repeat("=", 80), html, strings.Repeat("=", 80))
}

// uploadCraCdn uploads the CDN files of a Create React App style build, whose
// JS and CSS files are located in web/build/static/js and web/build/static/css.
func uploadCraCdn(provider storage.StorageProvider, buildDir string, siteName string) (string, error) {
	domainUrl := ""
	for _, folder := range []string{"js", "css"} {
		relDir := filepath.Join("static", folder)

		filenames, err := util.ListFiles(filepath.Join(buildDir, relDir))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}

		folderUrl, err := uploadFolder(provider, buildDir, relDir, filterFiles(filenames, folder, siteName), "static")
		if err != nil {
			return "", err
		}

		if folder == "js" {
			domainUrl = folderUrl
		}
	}
	return domainUrl, nil
}

// uploadNextStatic uploads the files of a Next.js build's static folder, which the pages load from /_next/static/,
// and returns the CDN's URL of /_next/.
func uploadNextStatic(provider storage.StorageProvider, staticDir string) (string, error) {
	domainUrl := ""
	err := filepath.WalkDir(staticDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(staticDir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		objectKey := "_next/static/" + filepath.ToSlash(rel)
		fileUrl, err := provider.PutObject("Built-in-Untracked", "", objectKey, bytes.NewBuffer(data))
		if err != nil {
			return err
		}
		index := strings.Index(fileUrl, "/_next/static/")
		if index == -1 {
			return fmt.Errorf("uploadNextStatic() error, fileUrl should contain \"/_next/static/\", fileUrl = %s", fileUrl)
		}
		domainUrl = fileUrl[:index+len("/_next/")]
		fmt.Printf("uploadNextStatic(): [/%s] -> [%s]\n", objectKey, fileUrl)
		return nil
	})
	if err != nil {
		return "", err
	}
	if domainUrl == "" {
		return "", fmt.Errorf("gitUploadCdn() error, no CDN file is uploaded in folder: %s", staticDir)
	}
	return domainUrl, nil
}

// uploadNextCdn uploads the static files of a Next.js server build: the build in web/build that web/build/server.txt
// names. The pages are rendered by the site's own server, so there is no index.html to rewrite: the CDN's URL of
// /_next/ goes to cdn.txt beside the build's server.js for the site to use.
func uploadNextCdn(provider storage.StorageProvider, buildDir string) error {
	serverName, err := os.ReadFile(filepath.Join(buildDir, "server.txt"))
	if err != nil {
		return err
	}
	serverDir := filepath.Join(buildDir, strings.TrimSpace(string(serverName)))

	staticDir := ""
	for _, distDir := range []string{".next-build", ".next"} {
		if util.FileExist(filepath.Join(serverDir, distDir, "static")) {
			staticDir = filepath.Join(serverDir, distDir, "static")
			break
		}
	}
	if staticDir == "" {
		return fmt.Errorf("uploadNextCdn() error, no .next-build/static or .next/static in [%s]", serverDir)
	}

	domainUrl, err := uploadNextStatic(provider, staticDir)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(serverDir, "cdn.txt"), []byte(domainUrl), 0o644)
}

// uploadNextExportCdn uploads the static files of a Next.js static export (web/build/_next/static) and points the
// exported pages (.html) and router payloads (.txt) at them. Turbopack's runtime is told to load the chunks it adds
// later from the CDN too.
func uploadNextExportCdn(provider storage.StorageProvider, buildDir string) error {
	domainUrl, err := uploadNextStatic(provider, filepath.Join(buildDir, "_next", "static"))
	if err != nil {
		return err
	}

	oldPrefix := []byte(`"/_next/static/`)
	newPrefix := []byte(`"` + domainUrl + "static/")
	script := []byte(`<script>self.TURBOPACK_CHUNK_BASE_PATH="` + domainUrl + `"</script>`)
	return filepath.WalkDir(buildDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path == filepath.Join(buildDir, "_next") {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".html" && ext != ".txt" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		updated := bytes.ReplaceAll(data, oldPrefix, newPrefix)
		if ext == ".html" && !bytes.Contains(updated, []byte("TURBOPACK_CHUNK_BASE_PATH")) {
			if index := bytes.Index(updated, []byte("<head>")); index >= 0 {
				end := index + len("<head>")
				updated = append(updated[:end:end], append(script, updated[end:]...)...)
			}
		}
		if bytes.Equal(updated, data) {
			return nil
		}
		return os.WriteFile(path, updated, entry.Type().Perm()|0o644)
	})
}

// uploadViteCdn uploads the CDN files of a Vite style build, whose JS, CSS and
// other assets are all located in web/build/assets.
func uploadViteCdn(provider storage.StorageProvider, buildDir string) (string, error) {
	filenames, err := util.ListFiles(filepath.Join(buildDir, "assets"))
	if err != nil {
		return "", err
	}

	return uploadFolder(provider, buildDir, "assets", filenames, "assets")
}

func gitUploadCdn(providerName string, siteName string) error {
	if providerName == "" {
		return nil
	}

	path := GetRepoPath(siteName)
	buildDir := filepath.Join(path, "web/build")
	if os.Getenv("frontendCdnUrl") != "" && util.FileExist(filepath.Join(buildDir, "package.json")) {
		fmt.Printf("gitUploadCdn(): siteName = [%s] is served from frontendCdnUrl, skipped\n", siteName)
		return nil
	}

	fmt.Printf("gitUploadCdn(): providerName = [%s], siteName = [%s], buildDir = [%s], casdoorOrganization = [%s], casdoorApplication = [%s]\n",
		providerName, siteName, buildDir, beego.AppConfig.String("casdoorOrganization"), beego.AppConfig.String("casdoorApplication"))

	provider, err := storage.GetStorageProvider(providerName)
	if err != nil {
		return err
	}

	if util.FileExist(filepath.Join(buildDir, "server.txt")) {
		return uploadNextCdn(provider, buildDir)
	}
	if util.FileExist(filepath.Join(buildDir, "_next", "static")) {
		return uploadNextExportCdn(provider, buildDir)
	}

	rootDir := ""
	domainUrl := ""
	if util.FileExist(filepath.Join(buildDir, "static", "js")) || util.FileExist(filepath.Join(buildDir, "static", "css")) {
		rootDir = "static"
		domainUrl, err = uploadCraCdn(provider, buildDir, siteName)
	} else if util.FileExist(filepath.Join(buildDir, "assets")) {
		rootDir = "assets"
		domainUrl, err = uploadViteCdn(provider, buildDir)
	} else {
		return fmt.Errorf("gitUploadCdn() error, neither [%s] nor [%s] exists", filepath.Join(buildDir, "static"), filepath.Join(buildDir, "assets"))
	}
	if err != nil {
		return err
	}

	if domainUrl == "" {
		return fmt.Errorf("gitUploadCdn() error, no CDN file is uploaded in folder: %s", filepath.Join(buildDir, rootDir))
	}

	updateHtml(domainUrl, buildDir, rootDir)
	return nil
}
