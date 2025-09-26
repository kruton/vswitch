package vswitch

import (
	"net"
	"testing"
	"time"
)

// mockConn implements net.Conn for testing
type mockConnSwitch struct {
	readData  []byte
	readPos   int
	writeData []byte
	closed    bool
	addr      net.Addr
}

func (m *mockConnSwitch) Read(b []byte) (int, error) {
	if m.closed {
		return 0, net.ErrClosed
	}
	if m.readPos >= len(m.readData) {
		return 0, nil
	}
	n := copy(b, m.readData[m.readPos:])
	m.readPos += n
	return n, nil
}

func (m *mockConnSwitch) Write(b []byte) (int, error) {
	if m.closed {
		return 0, net.ErrClosed
	}
	m.writeData = append(m.writeData, b...)
	return len(b), nil
}

func (m *mockConnSwitch) Close() error {
	m.closed = true
	return nil
}

func (m *mockConnSwitch) LocalAddr() net.Addr  { return m.addr }
func (m *mockConnSwitch) RemoteAddr() net.Addr { return m.addr }

func (m *mockConnSwitch) SetDeadline(t time.Time) error      { return nil }
func (m *mockConnSwitch) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConnSwitch) SetWriteDeadline(t time.Time) error { return nil }

// mockAddr implements net.Addr for testing
type mockAddrSwitch struct {
	network string
	address string
}

func (m *mockAddrSwitch) Network() string { return m.network }
func (m *mockAddrSwitch) String() string  { return m.address }

func TestNewVirtualSwitch(t *testing.T) {
	ports := []int{8080, 8081}
	sw := NewVirtualSwitch(ports)

	if len(sw.ports) != 2 {
		t.Errorf("Expected 2 ports, got %d", len(sw.ports))
	}

	if sw.ports[0] != 8080 || sw.ports[1] != 8081 {
		t.Errorf("Expected ports [8080, 8081], got %v", sw.ports)
	}

	if sw.macTimeout == 0 {
		t.Errorf("Expected MAC timeout to be set")
	}

	if sw.shutdown == nil {
		t.Errorf("Expected shutdown channel to be initialized")
	}
}

func TestVirtualSwitchStop(t *testing.T) {
	ports := []int{8080}
	sw := NewVirtualSwitch(ports)

	// Test stopping when not running
	sw.Stop()

	// Should not panic
}

func TestVirtualSwitchGetStats(t *testing.T) {
	ports := []int{8080}
	sw := NewVirtualSwitch(ports)

	stats := sw.GetStats()

	expectedFields := []string{
		"connections", "total_frames", "broadcast_frames",
		"unicast_frames", "dropped_frames", "mac_entries",
	}

	for _, field := range expectedFields {
		if _, exists := stats[field]; !exists {
			t.Errorf("Expected stats field '%s' to exist", field)
		}
	}

	// Check initial values
	if stats["connections"] != 0 {
		t.Errorf("Expected connections to be 0, got %v", stats["connections"])
	}

	if stats["total_frames"] != uint64(0) {
		t.Errorf("Expected total_frames to be 0, got %v", stats["total_frames"])
	}
}

func TestProcessFrame(t *testing.T) {
	ports := []int{8080}
	sw := NewVirtualSwitch(ports)

	// Create mock connections
	mockConn1 := &mockConnSwitch{
		addr: &mockAddrSwitch{network: "tcp", address: "127.0.0.1:9001"},
	}
	mockConn2 := &mockConnSwitch{
		addr: &mockAddrSwitch{network: "tcp", address: "127.0.0.1:9002"},
	}

	conn1 := NewConnection("conn1", mockConn1)
	conn2 := NewConnection("conn2", mockConn2)

	// Add connections to switch
	sw.connections.Store("conn1", conn1)
	sw.connections.Store("conn2", conn2)

	// Test broadcast frame
	broadcastFrame := &EthernetFrame{
		DestMAC:   net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		SrcMAC:    net.HardwareAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
		EtherType: 0x0800,
		Raw:       make([]byte, 64),
	}

	// Process broadcast frame
	_ = sw.processFrame(broadcastFrame, conn1)

	// Check that frame was written to conn2 but not back to conn1
	if len(mockConn1.writeData) > 0 {
		t.Errorf("Expected no data written back to sender connection")
	}

	if len(mockConn2.writeData) == 0 {
		t.Errorf("Expected frame to be forwarded to other connection")
	}

	// Check statistics
	stats := sw.GetStats()
	if stats["broadcast_frames"] != uint64(1) {
		t.Errorf("Expected 1 broadcast frame, got %v", stats["broadcast_frames"])
	}

	if stats["total_frames"] != uint64(1) {
		t.Errorf("Expected 1 total frame, got %v", stats["total_frames"])
	}
}

func TestLearnMAC(t *testing.T) {
	ports := []int{8080}
	sw := NewVirtualSwitch(ports)

	mockConn := &mockConnSwitch{
		addr: &mockAddrSwitch{network: "tcp", address: "127.0.0.1:9001"},
	}
	conn := NewConnection("conn1", mockConn)

	srcMAC := net.HardwareAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}

	// Learn MAC address
	sw.learnMAC(srcMAC, conn)

	// Check that MAC was learned
	if entry, exists := sw.macTable.Load(srcMAC.String()); !exists {
		t.Errorf("Expected MAC %s to be learned", srcMAC.String())
	} else {
		macEntry := entry.(*MACEntry)
		if macEntry.Connection != conn {
			t.Errorf("Expected MAC entry to point to correct connection")
		}
	}

	// Check statistics
	stats := sw.GetStats()
	if stats["mac_entries"] != 1 {
		t.Errorf("Expected 1 MAC entry, got %v", stats["mac_entries"])
	}
}

func TestForwardFrame(t *testing.T) {
	ports := []int{8080}
	sw := NewVirtualSwitch(ports)

	mockConn1 := &mockConnSwitch{
		addr: &mockAddrSwitch{network: "tcp", address: "127.0.0.1:9001"},
	}
	mockConn2 := &mockConnSwitch{
		addr: &mockAddrSwitch{network: "tcp", address: "127.0.0.1:9002"},
	}

	conn1 := NewConnection("conn1", mockConn1)
	conn2 := NewConnection("conn2", mockConn2)

	// Learn a MAC on conn2 (use unicast MAC - even first byte)
	destMAC := net.HardwareAddr{0x02, 0x02, 0x03, 0x04, 0x05, 0x06}
	sw.learnMAC(destMAC, conn2)

	// Create unicast frame
	unicastFrame := &EthernetFrame{
		DestMAC:   destMAC,
		SrcMAC:    net.HardwareAddr{0x08, 0x08, 0x09, 0x0a, 0x0b, 0x0c}, // Also use unicast source MAC
		EtherType: 0x0800,
		Raw:       make([]byte, 64),
	}

	// Process frame (which will forward it)
	_ = sw.processFrame(unicastFrame, conn1)

	// Check that frame was written to conn2
	if len(mockConn2.writeData) == 0 {
		t.Errorf("Expected frame to be written to destination connection")
	}

	// Check statistics
	stats := sw.GetStats()
	if stats["unicast_frames"] != uint64(1) {
		t.Errorf("Expected 1 unicast frame, got %v", stats["unicast_frames"])
	}
}

func TestFloodFrame(t *testing.T) {
	ports := []int{8080}
	sw := NewVirtualSwitch(ports)

	mockConn1 := &mockConnSwitch{
		addr: &mockAddrSwitch{network: "tcp", address: "127.0.0.1:9001"},
	}
	mockConn2 := &mockConnSwitch{
		addr: &mockAddrSwitch{network: "tcp", address: "127.0.0.1:9002"},
	}
	mockConn3 := &mockConnSwitch{
		addr: &mockAddrSwitch{network: "tcp", address: "127.0.0.1:9003"},
	}

	conn1 := NewConnection("conn1", mockConn1)
	conn2 := NewConnection("conn2", mockConn2)
	conn3 := NewConnection("conn3", mockConn3)

	// Add connections to switch
	sw.connections.Store("conn1", conn1)
	sw.connections.Store("conn2", conn2)
	sw.connections.Store("conn3", conn3)

	// Create frame to flood
	frame := &EthernetFrame{
		DestMAC:   net.HardwareAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
		SrcMAC:    net.HardwareAddr{0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c},
		EtherType: 0x0800,
		Raw:       make([]byte, 64),
	}

	// Flood frame from conn1 (should reach conn2 and conn3)
	_ = sw.floodFrame(frame, conn1)

	// Check that frame was not written back to sender
	if len(mockConn1.writeData) > 0 {
		t.Errorf("Expected no data written back to sender connection")
	}

	// Check that frame was written to other connections
	if len(mockConn2.writeData) == 0 {
		t.Errorf("Expected frame to be flooded to conn2")
	}

	if len(mockConn3.writeData) == 0 {
		t.Errorf("Expected frame to be flooded to conn3")
	}
}

func TestCleanupConnection(t *testing.T) {
	ports := []int{8080}
	sw := NewVirtualSwitch(ports)

	mockConn := &mockConnSwitch{
		addr: &mockAddrSwitch{network: "tcp", address: "127.0.0.1:9001"},
	}
	conn := NewConnection("conn1", mockConn)

	// Add connection to switch
	sw.connections.Store("conn1", conn)

	// Learn a MAC on this connection
	srcMAC := net.HardwareAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	sw.learnMAC(srcMAC, conn)

	// Cleanup connection
	sw.cleanupConnection(conn)

	// Check that connection was removed
	if _, exists := sw.connections.Load("conn1"); exists {
		t.Errorf("Expected connection to be removed from connections map")
	}

	// Check that MAC entry was removed
	if _, exists := sw.macTable.Load(srcMAC.String()); exists {
		t.Errorf("Expected MAC entry to be removed from MAC table")
	}
}

func TestCleanupStaleMACs(t *testing.T) {
	ports := []int{8080}
	sw := NewVirtualSwitch(ports)

	mockConn := &mockConnSwitch{
		addr: &mockAddrSwitch{network: "tcp", address: "127.0.0.1:9001"},
	}
	conn := NewConnection("conn1", mockConn)

	// Learn a MAC
	srcMAC := net.HardwareAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	sw.learnMAC(srcMAC, conn)

	// Manually set MAC entry to be old (more than MAC aging time)
	if entry, exists := sw.macTable.Load(srcMAC.String()); exists {
		macEntry := entry.(*MACEntry)
		macEntry.LearnedAt = time.Now().Add(-10 * time.Minute) // Old entry
	}

	// Cleanup stale MACs
	sw.cleanupStaleMACs()

	// Check that MAC entry was removed
	if _, exists := sw.macTable.Load(srcMAC.String()); exists {
		t.Errorf("Expected stale MAC entry to be removed")
	}
}
