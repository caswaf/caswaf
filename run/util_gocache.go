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
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/casbin/caswaf/util"
)

const (
	goCacheCleanInterval = time.Hour
	// the go command itself only trims the cache entries unused for 5 days, which piles up tens of GB when
	// every pull of every site rebuilds it (and "go run" caches the linked executable too since Go 1.24)
	goCacheMaxUnusedTime = 2 * 24 * time.Hour
)

// getGoCacheDir returns the build cache of the go command run by the sites' bats,
// e.g. "C:\Users\Administrator\AppData\Local\go-build", or "" if there is none
func getGoCacheDir() string {
	out, err := exec.Command("go", "env", "GOCACHE").Output()
	if err == nil {
		// "off" if GOCACHE=off
		res := strings.TrimSpace(string(out))
		if filepath.IsAbs(res) {
			return res
		}
		return ""
	}

	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "go-build")
}

// cleanGoCache removes the cache entries not used since the cutoff, like the trimming of the go command does:
// the go command refreshes the mtime of an entry when using it, so a recent mtime means the entry is still in use
func cleanGoCache(dir string, cutoff time.Time) (int, int64, error) {
	if !util.FileExist(dir) {
		return 0, 0, nil
	}

	count := 0
	size := int64(0)
	// the entries are in the subdirectories "00" to "ff", the files like "README" and "trim.txt" at the top are kept
	for i := 0; i < 256; i++ {
		subDir := filepath.Join(dir, fmt.Sprintf("%02x", i))
		entries, err := os.ReadDir(subDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return count, size, err
		}

		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil || info.IsDir() || !info.ModTime().Before(cutoff) {
				continue
			}

			err = os.Remove(filepath.Join(subDir, entry.Name()))
			if err != nil {
				continue
			}
			count++
			size += info.Size()
		}
	}
	return count, size, nil
}

func getDirSize(dir string) int64 {
	size := int64(0)
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err == nil && !d.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

// cleanGoBuildTempDirs removes the work directories like "%TEMP%\go-build1234567890" older than the cutoff,
// which "go run" leaves behind when it is killed (e.g. by taskkill /F on a restart) instead of exiting by itself
func cleanGoBuildTempDirs(cutoff time.Time) (int, int64, error) {
	tempDir := os.Getenv("GOTMPDIR")
	if tempDir == "" {
		tempDir = os.TempDir()
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return 0, 0, err
	}

	count := 0
	size := int64(0)
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "go-build") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}

		// the directory of a site running for long is kept, as its executable is locked by the running process
		path := filepath.Join(tempDir, entry.Name())
		dirSize := getDirSize(path)
		err = os.RemoveAll(path)
		if err != nil {
			continue
		}
		count++
		size += dirSize
	}
	return count, size, nil
}

func cleanGoBuildOnce() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[%s] Recovered from cleanGoBuildOnce() panic: %v\n", util.GetCurrentTime(), r)
		}
	}()

	cutoff := time.Now().Add(-goCacheMaxUnusedTime)

	cacheDir := getGoCacheDir()
	count, size, err := cleanGoCache(cacheDir, cutoff)
	if err != nil {
		fmt.Printf("[%s] cleanGoCache() error, dir = %s: %v\n", util.GetCurrentTime(), cacheDir, err)
	}
	if count > 0 {
		fmt.Printf("[%s] cleanGoCache(): removed %d entries (%.2f GB) from %s\n", util.GetCurrentTime(), count, float64(size)/1e9, cacheDir)
	}

	count, size, err = cleanGoBuildTempDirs(cutoff)
	if err != nil {
		fmt.Printf("[%s] cleanGoBuildTempDirs() error: %v\n", util.GetCurrentTime(), err)
	}
	if count > 0 {
		fmt.Printf("[%s] cleanGoBuildTempDirs(): removed %d directories (%.2f GB)\n", util.GetCurrentTime(), count, float64(size)/1e9)
	}
}

// StartCleanGoBuildLoop periodically frees the disk taken by the old builds of the sites run by "go run main.go"
func StartCleanGoBuildLoop() {
	go func() {
		for {
			cleanGoBuildOnce()

			time.Sleep(goCacheCleanInterval)
		}
	}()
}
