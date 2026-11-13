// Copyright 2025 The casbin Authors. All Rights Reserved.
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
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

const (
	// EnvSupervisorKey is the environment variable key to detect if running under supervisor
	EnvSupervisorKey = "CASWAF_SUPERVISED"
	// MaxRestarts is the maximum number of restarts within the restart window
	MaxRestarts = 5
	// RestartWindow is the time window for counting restarts
	RestartWindow = 5 * time.Minute
	// RestartDelay is the delay before restarting after a crash
	RestartDelay = 2 * time.Second
)

// InitSelfGuard initializes the self-recovery mechanism
// If not already supervised, it starts a supervisor process and exits
// If already supervised, it does nothing and returns
func InitSelfGuard() {
	// Check if we're already supervised
	if os.Getenv(EnvSupervisorKey) == "1" {
		// Already supervised, just return and continue normal execution
		return
	}
	
	// Start as supervisor
	err := runSupervisor()
	if err != nil {
		fmt.Printf("Supervisor error: %v\n", err)
		FlushLog()
		os.Exit(1)
	}
	// If we get here, supervisor exited cleanly
	FlushLog()
	os.Exit(0)
}

// runSupervisor starts the supervisor that monitors and restarts the main process
func runSupervisor() error {
	fmt.Println("Starting CasWAF with auto-recovery mechanism...")
	
	restartTimes := []time.Time{}
	
	for {
		// Clean up old restart times outside the window
		now := time.Now()
		validRestarts := []time.Time{}
		for _, t := range restartTimes {
			if now.Sub(t) < RestartWindow {
				validRestarts = append(validRestarts, t)
			}
		}
		restartTimes = validRestarts
		
		// Check if we've exceeded max restarts
		if len(restartTimes) >= MaxRestarts {
			return fmt.Errorf("exceeded maximum restart limit (%d restarts in %v), stopping supervisor", MaxRestarts, RestartWindow)
		}
		
		// Start the child process
		cmd := exec.Command(os.Args[0], os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		
		// Set environment variable to indicate supervised process
		cmd.Env = append(os.Environ(), fmt.Sprintf("%s=1", EnvSupervisorKey))
		
		// Start the process
		if err := cmd.Start(); err != nil {
			fmt.Printf("Failed to start process: %v\n", err)
			return err
		}
		
		processStartTime := time.Now()
		fmt.Printf("Started supervised process with PID: %d\n", cmd.Process.Pid)
		
		// Setup signal handling to forward signals to child
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		
		// Wait for process completion or signal
		doneChan := make(chan error, 1)
		go func() {
			doneChan <- cmd.Wait()
		}()
		
		select {
		case err := <-doneChan:
			// Process exited
			exitTime := time.Now()
			uptime := exitTime.Sub(processStartTime)
			
			if err != nil {
				fmt.Printf("Process crashed after %v: %v\n", uptime, err)
				
				// Record this restart
				restartTimes = append(restartTimes, time.Now())
				
				fmt.Printf("Waiting %v before restarting... (restart %d/%d)\n", 
					RestartDelay, len(restartTimes), MaxRestarts)
				time.Sleep(RestartDelay)
				
				// Continue to restart
				continue
			} else {
				// Clean exit
				fmt.Println("Process exited cleanly")
				return nil
			}
			
		case sig := <-sigChan:
			// Received shutdown signal, forward to child
			fmt.Printf("Received signal %v, forwarding to child process...\n", sig)
			if cmd.Process != nil {
				cmd.Process.Signal(sig)
			}
			// Wait for child to exit
			<-doneChan
			return nil
		}
	}
}
