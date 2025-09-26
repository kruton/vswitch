package vswitch

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

// MACEntry represents an entry in the MAC learning table
type MACEntry struct {
	Connection *Connection
	LearnedAt  time.Time
}

// VirtualSwitch implements a software Ethernet switch with MAC learning
type VirtualSwitch struct {
	// MAC learning table
	macTable sync.Map // map[string]*MACEntry

	// Active connections
	connections sync.Map // map[string]*Connection

	// Configuration
	macTimeout time.Duration
	ports      []int

	// Statistics
	totalFrames    uint64
	broadcastFrames uint64
	unicastFrames  uint64
	droppedFrames  uint64

	// Control
	shutdown chan bool
	wg       sync.WaitGroup
}

// NewVirtualSwitch creates a new virtual switch instance
func NewVirtualSwitch(ports []int) *VirtualSwitch {
	return &VirtualSwitch{
		ports:      ports,
		macTimeout: 300 * time.Second, // 5 minutes default MAC timeout
		shutdown:   make(chan bool),
	}
}

// Start starts the virtual switch on all configured ports
func (vs *VirtualSwitch) Start() error {
	log.Printf("Starting virtual switch on ports: %v", vs.ports)

	for _, port := range vs.ports {
		vs.wg.Add(1)
		go vs.listenOnPort(port)
	}

	// Start MAC table cleanup routine
	vs.wg.Add(1)
	go vs.macTableCleanup()

	return nil
}

// Stop stops the virtual switch and closes all connections
func (vs *VirtualSwitch) Stop() {
	log.Printf("Stopping virtual switch")

	close(vs.shutdown)

	// Close all connections
	vs.connections.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*Connection); ok {
			_ = conn.Close()
		}
		return true
	})

	vs.wg.Wait()
	log.Printf("Virtual switch stopped")
}

// listenOnPort starts a listener on the specified port
func (vs *VirtualSwitch) listenOnPort(port int) {
	defer vs.wg.Done()

	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		log.Printf("Failed to listen on port %d: %v", port, err)
		return
	}
	defer func() { _ = listener.Close() }()

	log.Printf("Listening on port %d", port)

	for {
		select {
		case <-vs.shutdown:
			return
		default:
		}

		// Set accept timeout to allow periodic shutdown checks
		if tcpListener, ok := listener.(*net.TCPListener); ok {
			_ = tcpListener.SetDeadline(time.Now().Add(1 * time.Second))
		}

		conn, err := listener.Accept()
		if err != nil {
			// Check if it's a timeout (expected for shutdown checking)
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			log.Printf("Failed to accept connection on port %d: %v", port, err)
			continue
		}

		// Generate connection ID
		connID := fmt.Sprintf("%s-%d", conn.RemoteAddr().String(), port)
		connection := NewConnection(connID, conn)

		// Store the connection
		vs.connections.Store(connID, connection)
		log.Printf("New connection: %s", connection.String())

		// Handle the connection
		vs.wg.Add(1)
		go vs.handleConnection(connection)
	}
}

// handleConnection handles a single VM connection
func (vs *VirtualSwitch) handleConnection(conn *Connection) {
	defer vs.wg.Done()
	defer func() {
		vs.cleanupConnection(conn)
	}()

	log.Printf("Handling connection: %s", conn.ID)

	for {
		select {
		case <-vs.shutdown:
			return
		default:
		}

		// Set read timeout to allow periodic shutdown checks
		_ = conn.Conn.SetReadDeadline(time.Now().Add(1 * time.Second))

		frame, err := conn.ReadFrame()
		if err != nil {
			// Check if it's a timeout (expected for shutdown checking)
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			log.Printf("Connection %s read error: %v", conn.ID, err)
			return
		}

		// Process the frame
		if err := vs.processFrame(frame, conn); err != nil {
			log.Printf("Error processing frame from %s: %v", conn.ID, err)
			vs.droppedFrames++
		}
	}
}

// processFrame processes an incoming Ethernet frame
func (vs *VirtualSwitch) processFrame(frame *EthernetFrame, sourceConn *Connection) error {
	vs.totalFrames++

	// Learn the source MAC address
	vs.learnMAC(frame.SrcMAC, sourceConn)

	// Forward the frame based on destination MAC
	if frame.IsBroadcast() || frame.IsMulticast() {
		vs.broadcastFrames++
		return vs.floodFrame(frame, sourceConn)
	}
	vs.unicastFrames++
	return vs.forwardFrame(frame, sourceConn)
}

// learnMAC learns or updates a MAC address in the learning table
func (vs *VirtualSwitch) learnMAC(mac net.HardwareAddr, conn *Connection) {
	macStr := mac.String()
	entry := &MACEntry{
		Connection: conn,
		LearnedAt:  time.Now(),
	}

	vs.macTable.Store(macStr, entry)
	log.Printf("Learned MAC %s on connection %s", macStr, conn.ID)
}

// forwardFrame forwards a unicast frame to the destination
func (vs *VirtualSwitch) forwardFrame(frame *EthernetFrame, sourceConn *Connection) error {
	destMAC := frame.DestMAC.String()

	// Look up destination in MAC table
	if entryInterface, found := vs.macTable.Load(destMAC); found {
		entry := entryInterface.(*MACEntry)

		// Don't forward back to source
		if entry.Connection.ID == sourceConn.ID {
			return nil
		}

		// Forward to specific destination
		if !entry.Connection.IsClosed() {
			if err := entry.Connection.WriteFrame(frame); err != nil {
				log.Printf("Failed to forward frame to %s: %v", entry.Connection.ID, err)
				return err
			}
			log.Printf("Forwarded unicast frame %s -> %s via %s",
				frame.SrcMAC.String(), destMAC, entry.Connection.ID)
		}
	} else {
		// Unknown destination - flood the frame
		log.Printf("Unknown destination %s, flooding frame", destMAC)
		return vs.floodFrame(frame, sourceConn)
	}

	return nil
}

// floodFrame floods a frame to all connections except the source
func (vs *VirtualSwitch) floodFrame(frame *EthernetFrame, sourceConn *Connection) error {
	var errors []error

	vs.connections.Range(func(key, value interface{}) bool {
		conn := value.(*Connection)

		// Don't flood back to source
		if conn.ID == sourceConn.ID {
			return true
		}

		// Skip closed connections
		if conn.IsClosed() {
			return true
		}

		if err := conn.WriteFrame(frame); err != nil {
			log.Printf("Failed to flood frame to %s: %v", conn.ID, err)
			errors = append(errors, err)
		}

		return true
	})

	if len(errors) > 0 {
		log.Printf("Flooding completed with %d errors", len(errors))
	} else {
		log.Printf("Flooded %s frame from %s to all connections",
			map[bool]string{true: "broadcast", false: "multicast"}[frame.IsBroadcast()],
			frame.SrcMAC.String())
	}

	return nil
}

// cleanupConnection cleans up resources when a connection is closed
func (vs *VirtualSwitch) cleanupConnection(conn *Connection) {
	log.Printf("Cleaning up connection: %s", conn.ID)

	// Remove connection from active connections
	vs.connections.Delete(conn.ID)

	// Clean MAC entries for this connection
	vs.macTable.Range(func(key, value interface{}) bool {
		entry := value.(*MACEntry)
		if entry.Connection.ID == conn.ID {
			vs.macTable.Delete(key)
			log.Printf("Removed MAC entry %s for connection %s", key.(string), conn.ID)
		}
		return true
	})

	// Close the connection
	_ = conn.Close()
}

// macTableCleanup periodically cleans up stale MAC entries
func (vs *VirtualSwitch) macTableCleanup() {
	defer vs.wg.Done()

	ticker := time.NewTicker(30 * time.Second) // Clean every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-vs.shutdown:
			return
		case <-ticker.C:
			vs.cleanupStaleMACs()
		}
	}
}

// cleanupStaleMACs removes stale MAC entries from the learning table
func (vs *VirtualSwitch) cleanupStaleMACs() {
	now := time.Now()
	removed := 0

	vs.macTable.Range(func(key, value interface{}) bool {
		entry := value.(*MACEntry)

		// Remove entries that are too old or have closed connections
		if now.Sub(entry.LearnedAt) > vs.macTimeout || entry.Connection.IsClosed() {
			vs.macTable.Delete(key)
			removed++
		}

		return true
	})

	if removed > 0 {
		log.Printf("Cleaned up %d stale MAC entries", removed)
	}
}

// GetStats returns current switch statistics
func (vs *VirtualSwitch) GetStats() map[string]interface{} {
	connectionCount := 0
	macCount := 0

	vs.connections.Range(func(key, value interface{}) bool {
		connectionCount++
		return true
	})

	vs.macTable.Range(func(key, value interface{}) bool {
		macCount++
		return true
	})

	return map[string]interface{}{
		"total_frames":     vs.totalFrames,
		"broadcast_frames": vs.broadcastFrames,
		"unicast_frames":   vs.unicastFrames,
		"dropped_frames":   vs.droppedFrames,
		"connections":      connectionCount,
		"mac_entries":      macCount,
	}
}
