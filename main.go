package main

import (
	"flag"
	"fmt"
	"log"
	"log/syslog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	vswitch "vswitch-for-qemu/switch"
)

// getEnvOrDefault returns environment variable value or default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvIntOrDefault returns environment variable as int or default if not set/invalid
func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBoolOrDefault returns environment variable as bool or default if not set/invalid
func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

var (
	ports     = flag.String("ports", getEnvOrDefault("VSWITCH_PORTS", "9999,9998"), "Comma-separated list of ports (each port = isolated VLAN) [env: VSWITCH_PORTS]")
	statsPort = flag.Int("stats-port", getEnvIntOrDefault("VSWITCH_STATS_PORT", 0), "Port for statistics HTTP server (0 to disable) [env: VSWITCH_STATS_PORT]")
	daemon    = flag.Bool("daemon", getEnvBoolOrDefault("VSWITCH_DAEMON", false), "Run as daemon in background [env: VSWITCH_DAEMON]")
	pidFile   = flag.String("pid-file", getEnvOrDefault("VSWITCH_PID_FILE", "/tmp/vswitch.pid"), "PID file for daemon mode [env: VSWITCH_PID_FILE]")
	logFile   = flag.String("log-file", getEnvOrDefault("VSWITCH_LOG_FILE", ""), "Log file (empty for syslog) [env: VSWITCH_LOG_FILE]")
	stop      = flag.Bool("stop", false, "Stop running daemon")
	status    = flag.Bool("status", false, "Show daemon status")
	version   = flag.Bool("version", false, "Show version information")
)

const appVersion = "1.0.0"

// setupLogging configures logging based on daemon mode and log file settings
func setupLogging(logFile string, isDaemon bool) error {
	if logFile == "" {
		if isDaemon {
			// Use syslog for daemon mode when no log file specified
			syslogWriter, err := syslog.New(syslog.LOG_INFO|syslog.LOG_DAEMON, "vswitch")
			if err != nil {
				return fmt.Errorf("failed to connect to syslog: %v", err)
			}
			log.SetOutput(syslogWriter)
			log.SetFlags(0) // syslog handles timestamps and prefixes
		} else {
			// Use stdout for foreground mode when no log file specified
			log.SetOutput(os.Stdout)
			log.SetFlags(log.LstdFlags | log.Lshortfile)
		}
	} else {
		// Use file logging (handled by daemon.go for daemon mode)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}
	return nil
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Virtual Switch for QEMU VMs v%s\n\n", appVersion)
		fmt.Fprintf(os.Stderr, "A high-performance virtual Ethernet switch with isolated VLANs.\n")
		fmt.Fprintf(os.Stderr, "Each port creates a separate isolated virtual LAN.\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -ports 9999,9998\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -daemon -ports 8080,8081\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -stop\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -status\n", os.Args[0])
	}

	flag.Parse()

	if *version {
		fmt.Printf("Virtual Switch for QEMU VMs v%s\n", appVersion)
		os.Exit(0)
	}

	// Initialize daemon manager
	dm := vswitch.NewDaemonManager(*pidFile, *logFile)

	// Handle daemon control commands
	if *stop {
		if err := dm.Stop(); err != nil {
			log.Fatalf("Failed to stop daemon: %v", err)
		}
		fmt.Printf("Daemon stopped\n")
		os.Exit(0)
	}

	if *status {
		if dm.IsRunning() {
			pid := dm.GetPID()
			fmt.Printf("Daemon is running (PID: %d)\n", pid)
		} else {
			fmt.Printf("Daemon is not running\n")
		}
		os.Exit(0)
	}

	// Parse ports
	portList, err := parsePorts(*ports)
	if err != nil {
		log.Fatalf("Invalid ports specification: %v", err)
	}

	if len(portList) == 0 {
		log.Fatalf("No ports specified")
	}

	// Handle daemon mode
	if *daemon {
		// Remove daemon flag from args to prevent recursion
		args := []string{}
		for i, arg := range os.Args {
			if arg != "-daemon" {
				args = append(args, os.Args[i])
			}
		}
		if err := dm.Daemonize(args); err != nil {
			log.Fatalf("Failed to start daemon: %v", err)
		}
		fmt.Printf("Daemon started\n")
		os.Exit(0)
	}

	// Set up logging
	if err := setupLogging(*logFile, *daemon); err != nil {
		log.Fatalf("Failed to setup logging: %v", err)
	}
	log.Printf("Starting Virtual Switch v%s", appVersion)
	log.Printf("Configured VLANs on ports: %v", portList)

	// Create switch manager and add VLANs for each port
	sm := vswitch.NewSwitchManager()
	for _, port := range portList {
		if err := sm.AddVLAN(port); err != nil {
			log.Fatalf("Failed to create VLAN on port %d: %v", port, err)
		}
	}

	// Start all VLANs
	if err := sm.StartAll(); err != nil {
		log.Fatalf("Failed to start VLANs: %v", err)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start statistics reporting if enabled
	if *statsPort > 0 {
		go startStatsServer(sm, *statsPort)
	}

	// Start periodic statistics logging
	go logStatsPeriodically(sm, 60*time.Second)

	log.Printf("Virtual switch started with %d isolated VLANs. Press Ctrl+C to stop.", len(portList))

	// Wait for shutdown signal
	sig := <-sigChan
	log.Printf("Received signal %s, shutting down...", sig)

	// Graceful shutdown
	sm.StopAll()

	// Clean up daemon artifacts if running as daemon
	if filepath.Base(os.Args[0]) != "main" { // Simple check if running as daemon
		dm.Cleanup()
	}

	log.Printf("Virtual switch stopped")
}

// parsePorts parses a comma-separated list of port numbers
func parsePorts(portStr string) ([]int, error) {
	if portStr == "" {
		return nil, fmt.Errorf("empty port string")
	}

	portStrs := strings.Split(portStr, ",")
	ports := make([]int, 0, len(portStrs))

	for _, str := range portStrs {
		str = strings.TrimSpace(str)
		if str == "" {
			continue
		}

		port, err := strconv.Atoi(str)
		if err != nil {
			return nil, fmt.Errorf("invalid port '%s': %v", str, err)
		}

		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("port %d out of range (1-65535)", port)
		}

		ports = append(ports, port)
	}

	return ports, nil
}

// logStatsPeriodically logs switch statistics periodically
func logStatsPeriodically(sm *vswitch.SwitchManager, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		stats := sm.GetStats()
		log.Printf("Stats: %d VLANs, %d total connections, %d MAC entries, %d total frames (%d unicast, %d broadcast, %d dropped)",
			stats["vlan_count"], stats["total_connections"], stats["total_mac_entries"], stats["total_frames"],
			stats["unicast_frames"], stats["broadcast_frames"], stats["dropped_frames"])
	}
}

// startStatsServer starts a simple HTTP server for statistics (placeholder)
func startStatsServer(sm *vswitch.SwitchManager, port int) {
	// This is a placeholder for a future HTTP statistics endpoint
	// For now, we'll just log that it would be started
	log.Printf("Statistics server would be started on port %d (not implemented yet)", port)
}

