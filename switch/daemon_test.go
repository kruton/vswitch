package vswitch

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestNewDaemonManager(t *testing.T) {
	pidFile := "/tmp/test.pid"
	logFile := "/tmp/test.log"

	dm := NewDaemonManager(pidFile, logFile)

	if dm.pidFile != pidFile {
		t.Errorf("Expected pidFile '%s', got '%s'", pidFile, dm.pidFile)
	}

	if dm.logFile != logFile {
		t.Errorf("Expected logFile '%s', got '%s'", logFile, dm.logFile)
	}
}

func TestDaemonManagerIsRunning(t *testing.T) {
	// Create temporary PID file
	tmpDir, err := os.MkdirTemp("", "daemon_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	pidFile := filepath.Join(tmpDir, "test.pid")
	dm := NewDaemonManager(pidFile, "")

	// Should not be running initially
	if dm.IsRunning() {
		t.Errorf("Expected daemon to not be running initially")
	}

	// Write current process PID (should be running)
	currentPID := os.Getpid()
	err = os.WriteFile(pidFile, []byte(strconv.Itoa(currentPID)), 0644)
	if err != nil {
		t.Fatalf("Failed to write PID file: %v", err)
	}

	if !dm.IsRunning() {
		t.Errorf("Expected daemon to be running with current PID")
	}

	// Write invalid PID (should not be running)
	err = os.WriteFile(pidFile, []byte("99999999"), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid PID file: %v", err)
	}

	if dm.IsRunning() {
		t.Errorf("Expected daemon to not be running with invalid PID")
	}
}

func TestDaemonManagerGetPID(t *testing.T) {
	// Create temporary PID file
	tmpDir, err := os.MkdirTemp("", "daemon_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	pidFile := filepath.Join(tmpDir, "test.pid")
	dm := NewDaemonManager(pidFile, "")

	// Should return -1 for non-existent PID file
	if pid := dm.GetPID(); pid != -1 {
		t.Errorf("Expected PID -1 for non-existent file, got %d", pid)
	}

	// Write test PID
	testPID := 12345
	err = os.WriteFile(pidFile, []byte(strconv.Itoa(testPID)), 0644)
	if err != nil {
		t.Fatalf("Failed to write PID file: %v", err)
	}

	if pid := dm.GetPID(); pid != testPID {
		t.Errorf("Expected PID %d, got %d", testPID, pid)
	}
}

func TestDaemonManagerWriteReadPIDFile(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "daemon_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	pidFile := filepath.Join(tmpDir, "test.pid")
	dm := NewDaemonManager(pidFile, "")

	testPID := 12345

	// Test write
	err = dm.writePIDFile(testPID)
	if err != nil {
		t.Errorf("Unexpected error writing PID file: %v", err)
	}

	// Test read
	readPID, err := dm.readPIDFile()
	if err != nil {
		t.Errorf("Unexpected error reading PID file: %v", err)
	}

	if readPID != testPID {
		t.Errorf("Expected PID %d, got %d", testPID, readPID)
	}

	// Test read non-existent file
	_ = os.Remove(pidFile)
	_, err = dm.readPIDFile()
	if err == nil {
		t.Errorf("Expected error reading non-existent PID file")
	}
}

func TestDaemonManagerCleanup(t *testing.T) {
	// Create temporary PID file
	tmpDir, err := os.MkdirTemp("", "daemon_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	pidFile := filepath.Join(tmpDir, "test.pid")
	dm := NewDaemonManager(pidFile, "")

	// Write PID file
	err = dm.writePIDFile(12345)
	if err != nil {
		t.Fatalf("Failed to write PID file: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		t.Errorf("Expected PID file to exist before cleanup")
	}

	// Cleanup
	dm.Cleanup()

	// Verify file is removed
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Errorf("Expected PID file to be removed after cleanup")
	}
}

func TestDaemonManagerWritePIDFileCreatesDirectory(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "daemon_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Use nested directory that doesn't exist
	pidFile := filepath.Join(tmpDir, "subdir", "test.pid")
	dm := NewDaemonManager(pidFile, "")

	testPID := 12345

	// Should create directory structure
	err = dm.writePIDFile(testPID)
	if err != nil {
		t.Errorf("Unexpected error writing PID file with nested directory: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		t.Errorf("Expected PID file to be created in nested directory")
	}
}
