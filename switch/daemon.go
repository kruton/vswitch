package vswitch

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// DaemonManager handles daemonization and PID file management
type DaemonManager struct {
	pidFile string
	logFile string
}

// NewDaemonManager creates a new daemon manager
func NewDaemonManager(pidFile, logFile string) *DaemonManager {
	return &DaemonManager{
		pidFile: pidFile,
		logFile: logFile,
	}
}

// Daemonize starts the process as a daemon
func (dm *DaemonManager) Daemonize(args []string) error {
	// Check if already running
	if dm.IsRunning() {
		return fmt.Errorf("daemon already running (PID file: %s)", dm.pidFile)
	}

	// Prepare command for daemon process
	cmd := exec.Command(args[0], args[1:]...)

	// Set up environment for daemon
	cmd.Env = os.Environ()

	// Redirect output to log file if specified
	if dm.logFile != "" {
		// Ensure log directory exists
		logDir := filepath.Dir(dm.logFile)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %v", err)
		}

		logFile, err := os.OpenFile(dm.logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %v", err)
		}

		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}

	// Start the daemon process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %v", err)
	}

	// Write PID file
	if err := dm.writePIDFile(cmd.Process.Pid); err != nil {
		// Kill the process if we can't write PID file
		cmd.Process.Kill()
		return fmt.Errorf("failed to write PID file: %v", err)
	}

	log.Printf("Daemon started with PID %d", cmd.Process.Pid)
	return nil
}

// Stop stops the daemon process
func (dm *DaemonManager) Stop() error {
	pid, err := dm.readPIDFile()
	if err != nil {
		return fmt.Errorf("failed to read PID file: %v", err)
	}

	// Find process
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %v", pid, err)
	}

	// Send SIGTERM
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to process %d: %v", pid, err)
	}

	// Clean up PID file
	if err := os.Remove(dm.pidFile); err != nil {
		log.Printf("Warning: failed to remove PID file: %v", err)
	}

	log.Printf("Daemon stopped (PID %d)", pid)
	return nil
}

// IsRunning checks if the daemon is currently running
func (dm *DaemonManager) IsRunning() bool {
	pid, err := dm.readPIDFile()
	if err != nil {
		return false
	}

	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to test if process is alive
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// GetPID returns the PID of the running daemon, or -1 if not running
func (dm *DaemonManager) GetPID() int {
	pid, err := dm.readPIDFile()
	if err != nil {
		return -1
	}
	return pid
}

// writePIDFile writes the PID to the PID file
func (dm *DaemonManager) writePIDFile(pid int) error {
	// Ensure PID directory exists
	pidDir := filepath.Dir(dm.pidFile)
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return err
	}

	pidStr := strconv.Itoa(pid)
	return ioutil.WriteFile(dm.pidFile, []byte(pidStr), 0644)
}

// readPIDFile reads the PID from the PID file
func (dm *DaemonManager) readPIDFile() (int, error) {
	data, err := ioutil.ReadFile(dm.pidFile)
	if err != nil {
		return 0, err
	}

	pidStr := strings.TrimSpace(string(data))
	return strconv.Atoi(pidStr)
}

// Cleanup removes the PID file (called on daemon shutdown)
func (dm *DaemonManager) Cleanup() {
	if err := os.Remove(dm.pidFile); err != nil {
		log.Printf("Warning: failed to remove PID file: %v", err)
	}
}
