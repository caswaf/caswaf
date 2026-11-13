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

package util

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/beego/beego"
	"github.com/beego/beego/logs"
)

type rotatingFile struct {
	mu         sync.Mutex
	path       string
	maxSize    int64
	maxBackups int
	file       *os.File
	size       int64
}

func newRotatingFile(path string, maxSize int64, maxBackups int) (*rotatingFile, error) {
	rf := &rotatingFile{path: path, maxSize: maxSize, maxBackups: maxBackups}
	err := rf.open()
	if err != nil {
		return nil, err
	}
	return rf, nil
}

func (rf *rotatingFile) open() error {
	file, err := os.OpenFile(rf.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return err
	}

	rf.file = file
	rf.size = info.Size()
	return nil
}

func (rf *rotatingFile) rotate() error {
	rf.file.Close()
	os.Remove(fmt.Sprintf("%s.%d", rf.path, rf.maxBackups))
	for i := rf.maxBackups - 1; i >= 1; i-- {
		os.Rename(fmt.Sprintf("%s.%d", rf.path, i), fmt.Sprintf("%s.%d", rf.path, i+1))
	}
	if rf.maxBackups > 0 {
		os.Rename(rf.path, rf.path+".1")
	} else {
		os.Remove(rf.path)
	}
	return rf.open()
}

func (rf *rotatingFile) Write(p []byte) (int, error) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.file == nil {
		err := rf.open()
		if err != nil {
			return 0, err
		}
	}

	if rf.size > 0 && rf.size+int64(len(p)) > rf.maxSize {
		err := rf.rotate()
		if err != nil {
			return 0, err
		}
	}

	n, err := rf.file.Write(p)
	rf.size += int64(n)
	return n, err
}

var (
	logPipeWriter *os.File
	logDone       chan struct{}
	ansiRegex     = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
	timeRegex     = regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`)
)

const logAdapterName = "caswaf-console"

type consoleLogger struct{}

func (c *consoleLogger) Init(config string) error {
	return nil
}

func (c *consoleLogger) WriteMsg(when time.Time, msg string, level int) error {
	_, err := os.Stdout.Write([]byte(when.Format("2006/01/02 15:04:05.000") + " " + msg + "\n"))
	return err
}

func (c *consoleLogger) Destroy() {}

func (c *consoleLogger) Flush() {}

// InitLog copies everything CasWAF prints to the console into logs/caswaf.log, rotated by size
// (logMaxSize MB per file, logMaxBackups old files kept), and writes fatal crash output to logs/caswaf-crash.log
func InitLog() {
	logDir := beego.AppConfig.DefaultString("logDir", "logs")
	maxSize := int64(beego.AppConfig.DefaultInt("logMaxSize", 20)) * 1024 * 1024
	maxBackups := beego.AppConfig.DefaultInt("logMaxBackups", 4)

	err := os.MkdirAll(logDir, 0o755)
	if err != nil {
		fmt.Printf("InitLog() error: %v\n", err)
		return
	}

	initCrashOutput(filepath.Join(logDir, "caswaf-crash.log"))

	// a supervised child writes to the pipe it inherited from the supervisor, which already logs it
	if isSupervised {
		printStartLine()
		return
	}

	file, err := newRotatingFile(filepath.Join(logDir, "caswaf.log"), maxSize, maxBackups)
	if err != nil {
		fmt.Printf("InitLog() error: %v\n", err)
		return
	}

	r, w, err := os.Pipe()
	if err != nil {
		fmt.Printf("InitLog() error: %v\n", err)
		return
	}

	console := os.Stdout
	enableConsoleColor(console)
	logPipeWriter = w
	logDone = make(chan struct{})
	go copyLog(r, console, file)

	os.Stdout = w
	os.Stderr = w
	log.SetOutput(w)
	beeLogger := logs.GetBeeLogger()
	if beeLogger.DelLogger(logs.AdapterConsole) == nil {
		logs.Register(logAdapterName, func() logs.Logger { return &consoleLogger{} })
		beeLogger.SetLogger(logAdapterName)
	}

	printStartLine()
}

func printStartLine() {
	fmt.Printf("CasWAF started, pid = %d, ppid = %d, args = %s\n", os.Getpid(), os.Getppid(), strings.Join(os.Args, " "))
}

func copyLog(r io.Reader, console io.Writer, file io.Writer) {
	defer close(logDone)

	reader := bufio.NewReaderSize(r, 64*1024)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			console.Write([]byte(line))
			line = ansiRegex.ReplaceAllString(line, "")
			if !strings.HasSuffix(line, "\n") {
				line += "\n"
			}
			if !timeRegex.MatchString(line) {
				line = time.Now().Format("2006/01/02 15:04:05.000 ") + line
			}
			file.Write([]byte(line))
		}
		if err != nil {
			return
		}
	}
}

// FlushLog waits until everything printed so far is written to the log file, call it before os.Exit()
func FlushLog() {
	if logPipeWriter == nil {
		return
	}

	logPipeWriter.Close()
	select {
	case <-logDone:
	case <-time.After(3 * time.Second):
	}
}
