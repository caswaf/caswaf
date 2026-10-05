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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/beego/beego"
	"github.com/casbin/caswaf/util"
)

const (
	binaryBuildTimeout    = 30 * time.Minute
	binaryRetryInterval   = 30 * time.Minute
	binaryMaxUnusedTime   = 2 * 24 * time.Hour
	binaryDirName         = "_bin"
	binaryPointerFileTail = ".exe.txt"
)

var (
	binaryBuildLock    = &sync.Mutex{}
	binaryFailureMap   = map[string]binaryFailure{}
	binaryFailureMutex = &sync.Mutex{}
	// binaries that were built but could not be renamed into place yet, guarded by binaryBuildLock
	binaryBuiltMap = map[string]bool{}
)

type binaryFailure struct {
	time time.Time
	err  error
}

func isSelfSite(siteName string) bool {
	return getMappedName(siteName) == beego.AppConfig.String("dbName")
}

func canRunBinary(siteName string) bool {
	return !isSelfSite(siteName) && util.FileExist(filepath.Join(GetCodePath(siteName), "main.go"))
}

func shouldRunBinary(siteName string, useBinary bool) bool {
	return (useBinary || IsThinSite(siteName)) && canRunBinary(siteName)
}

func getBinaryDir(siteName string) string {
	return filepath.Join(beego.AppConfig.String("appDir"), binaryDirName, getOriginalName(siteName))
}

func getBinaryPointerPath(name string) string {
	name = getMappedName(name)
	return fmt.Sprintf("C:/Users/%s/Desktop/run/%s%s", username, name, binaryPointerFileTail)
}

func getBinaryPath(siteName string) (string, string, error) {
	path := GetCodePath(siteName)
	hash, err := gitGetLatestCommitHash(path)
	if err != nil {
		return "", "", err
	}

	fileName := fmt.Sprintf("%s_%s", getOriginalName(siteName), hash[:12])
	if IsThinSite(siteName) {
		return filepath.Join(getBinaryDir(siteName), fileName+".exe"), hash, nil
	}

	goDiff, err := runGitCommand(path, "diff", "HEAD", "--", "*.go", "go.mod", "go.sum")
	if err != nil {
		return "", "", err
	}
	if goDiff != "" {
		sum := sha256.Sum256([]byte(goDiff))
		fileName = fmt.Sprintf("%s_%s", fileName, hex.EncodeToString(sum[:])[:8])
	}

	return filepath.Join(getBinaryDir(siteName), fileName+".exe"), hash, nil
}

func buildBinary(repoPath string, binaryPath string) error {
	err := ensureFileFolderExists(filepath.Dir(binaryPath))
	if err != nil {
		return err
	}

	tmpPath := strings.TrimSuffix(binaryPath, ".exe") + ".building.exe"
	if binaryBuiltMap[binaryPath] && util.FileExist(tmpPath) {
		return renameBuiltBinary(tmpPath, binaryPath)
	}
	delete(binaryBuiltMap, binaryPath)
	_ = os.Remove(tmpPath)

	fmt.Printf("buildBinary(): [%s] -> [%s]\n", repoPath, binaryPath)

	ctx, cancel := context.WithTimeout(context.Background(), binaryBuildTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build", "-o", tmpPath, ".")
	cmd.Dir = repoPath
	var stderr bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("go build: timed out after %s", binaryBuildTimeout)
	}
	if err != nil {
		_ = os.Remove(tmpPath)
		output := strings.TrimSpace(stderr.String())
		if len(output) > 1000 {
			output = output[len(output)-1000:]
		}
		return fmt.Errorf("go build: %s, output: %s", err.Error(), output)
	}

	binaryBuiltMap[binaryPath] = true
	return renameBuiltBinary(tmpPath, binaryPath)
}

// A virus scanner (e.g. 360 on sg-machine) keeps a freshly built exe open for a while, so the rename is retried,
// and if it still fails the built file is kept for the next round instead of being built again.
func renameBuiltBinary(tmpPath string, binaryPath string) error {
	var err error
	for i := 0; i < 30; i++ {
		err = os.Rename(tmpPath, binaryPath)
		if err == nil {
			delete(binaryBuiltMap, binaryPath)
			return nil
		}
		time.Sleep(time.Second)
	}
	return err
}

// ensureBinary builds the site's code once per commit into a binary shared by all the sites of the same repo,
// and points the site's bat to it
func ensureBinary(siteName string) (string, error) {
	binaryPath, hash, err := getBinaryPath(siteName)
	if err != nil {
		return "", err
	}

	binaryBuildLock.Lock()
	defer binaryBuildLock.Unlock()

	if !util.FileExist(binaryPath) {
		binaryFailureMutex.Lock()
		failure, ok := binaryFailureMap[binaryPath]
		binaryFailureMutex.Unlock()
		if ok && time.Since(failure.time) < binaryRetryInterval {
			return "", fmt.Errorf("%s (failed at %s)", failure.err.Error(), failure.time.Format(time.RFC3339))
		}

		srcPath := GetCodePath(siteName)
		if IsThinSite(siteName) {
			srcPath, err = preparePristineSource(siteName, hash)
		}
		if err == nil {
			err = buildBinary(srcPath, binaryPath)
		}
		if err != nil {
			if !binaryBuiltMap[binaryPath] {
				binaryFailureMutex.Lock()
				binaryFailureMap[binaryPath] = binaryFailure{time: time.Now(), err: err}
				binaryFailureMutex.Unlock()
			}
			return "", err
		}
	}

	pointerPath := getBinaryPointerPath(siteName)
	err = ensureFileFolderExists(filepath.Dir(pointerPath))
	if err != nil {
		return "", err
	}
	content := filepath.FromSlash(binaryPath) + "\r\n"
	if readFileString(pointerPath) != content {
		err = os.WriteFile(pointerPath, []byte(content), 0644)
		if err != nil {
			return "", err
		}
	}

	return binaryPath, nil
}

func getBinaryBatContent(name string) string {
	return fmt.Sprintf("cd /d %s\r\nfor /f \"usebackq delims=\" %%%%i in (\"%s\") do \"%%%%i\"\r\n",
		filepath.FromSlash(GetRepoPath(name)), filepath.FromSlash(getBinaryPointerPath(name)))
}

func readFileString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func isBinaryBat(name string) bool {
	return readFileString(getBatPath(name)) == getBinaryBatContent(name)
}

// updateBinaryBatFile must not run while the old bat is running: cmd.exe reads a bat line by line,
// so it would go on with the new content after the program in the old bat exits
func updateBinaryBatFile(name string) error {
	batPath := getBatPath(name)
	err := ensureFileFolderExists(filepath.Dir(batPath))
	if err != nil {
		return err
	}

	if isBinaryBat(name) {
		return nil
	}

	fmt.Printf("updateBinaryBatFile(): [%s]\n", name)
	return os.WriteFile(batPath, []byte(getBinaryBatContent(name)), 0644)
}

func getRunningBinaryPath(siteName string) string {
	return strings.TrimSpace(readFileString(getBinaryPointerPath(siteName)))
}

func cleanOldBinaries(cutoff time.Time) (int, int64, error) {
	binaryRoot := filepath.Join(beego.AppConfig.String("appDir"), binaryDirName)
	if !util.FileExist(binaryRoot) {
		return 0, 0, nil
	}

	usedMap := map[string]bool{}
	pointerPaths, err := filepath.Glob(fmt.Sprintf("C:/Users/%s/Desktop/run/*%s", username, binaryPointerFileTail))
	if err != nil {
		return 0, 0, err
	}
	for _, pointerPath := range pointerPaths {
		usedMap[strings.ToLower(strings.TrimSpace(readFileString(pointerPath)))] = true
	}

	binaryPaths, err := filepath.Glob(filepath.Join(binaryRoot, "*", "*.exe"))
	if err != nil {
		return 0, 0, err
	}

	count := 0
	size := int64(0)
	for _, binaryPath := range binaryPaths {
		if usedMap[strings.ToLower(filepath.FromSlash(binaryPath))] {
			continue
		}

		info, err := os.Stat(binaryPath)
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}

		// a binary still run by a process is locked and can't be removed
		err = os.Remove(binaryPath)
		if err != nil {
			continue
		}
		count++
		size += info.Size()
	}
	return count, size, nil
}

func switchBatFile(name string, useBinary bool) error {
	if useBinary {
		return updateBinaryBatFile(name)
	}

	fmt.Printf("switchBatFile(): [%s] go run\n", name)
	return os.WriteFile(getBatPath(name), []byte(getGoRunBatContent(name)), 0644)
}
