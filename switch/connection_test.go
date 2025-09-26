package vswitch

import (
	"io"
	"net"
	"testing"
	"time"
)

// mockConn implements net.Conn for testing
type mockConn struct {
	readData  []byte
	readPos   int
	writeData []byte
	closed    bool
	addr      net.Addr
}

func (m *mockConn) Read(b []byte) (int, error) {
	if m.closed {
		return 0, net.ErrClosed
	}
	if m.readPos >= len(m.readData) {
		return 0, io.EOF
	}
	n := copy(b, m.readData[m.readPos:])
	m.readPos += n
	return n, nil
}

func (m *mockConn) Write(b []byte) (int, error) {
	if m.closed {
		return 0, net.ErrClosed
	}
	m.writeData = append(m.writeData, b...)
	return len(b), nil
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr  { return m.addr }
func (m *mockConn) RemoteAddr() net.Addr { return m.addr }

func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// mockAddr implements net.Addr for testing
type mockAddr struct {
	network string
	address string
}

func (m *mockAddr) Network() string { return m.network }
func (m *mockAddr) String() string  { return m.address }

func TestNewConnection(t *testing.T) {
	mockConn := &mockConn{
		addr: &mockAddr{network: "tcp", address: "127.0.0.1:8080"},
	}

	conn := NewConnection("test-conn", mockConn)

	if conn.ID != "test-conn" {
		t.Errorf("Expected ID 'test-conn', got '%s'", conn.ID)
	}

	if conn.Conn != mockConn {
		t.Errorf("Expected connection to match mock connection")
	}

	if conn.closed {
		t.Errorf("Expected connection to not be closed initially")
	}

	if conn.FramesSent != 0 || conn.FramesReceived != 0 {
		t.Errorf("Expected initial frame counts to be zero")
	}
}

func TestConnectionWriteFrame(t *testing.T) {
	mockConn := &mockConn{
		addr: &mockAddr{network: "tcp", address: "127.0.0.1:8080"},
	}

	conn := NewConnection("test-conn", mockConn)

	// Create test frame
	frameData := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x08, 0x00, 0x45, 0x00}
	frame := &EthernetFrame{
		Raw: frameData,
	}

	err := conn.WriteFrame(frame)
	if err != nil {
		t.Errorf("Unexpected error writing frame: %v", err)
	}

	// Check that frame length and data were written
	expectedLength := []byte{0x00, 0x00, 0x00, 0x10} // 16 bytes in big endian
	expectedData := append(expectedLength, frameData...)

	if len(mockConn.writeData) != len(expectedData) {
		t.Errorf("Expected %d bytes written, got %d", len(expectedData), len(mockConn.writeData))
	}

	for i, b := range expectedData {
		if i < len(mockConn.writeData) && mockConn.writeData[i] != b {
			t.Errorf("Expected byte %d to be 0x%02x, got 0x%02x", i, b, mockConn.writeData[i])
		}
	}

	// Check statistics
	if conn.FramesSent != 1 {
		t.Errorf("Expected 1 frame sent, got %d", conn.FramesSent)
	}

	if conn.BytesSent != uint64(len(frameData)) {
		t.Errorf("Expected %d bytes sent, got %d", len(frameData), conn.BytesSent)
	}
}

func TestConnectionClose(t *testing.T) {
	mockConn := &mockConn{
		addr: &mockAddr{network: "tcp", address: "127.0.0.1:8080"},
	}

	conn := NewConnection("test-conn", mockConn)

	if conn.IsClosed() {
		t.Errorf("Expected connection to not be closed initially")
	}

	err := conn.Close()
	if err != nil {
		t.Errorf("Unexpected error closing connection: %v", err)
	}

	if !conn.IsClosed() {
		t.Errorf("Expected connection to be closed after Close()")
	}

	if !mockConn.closed {
		t.Errorf("Expected underlying connection to be closed")
	}
}

func TestConnectionWriteFrameAfterClose(t *testing.T) {
	mockConn := &mockConn{
		addr: &mockAddr{network: "tcp", address: "127.0.0.1:8080"},
	}

	conn := NewConnection("test-conn", mockConn)
	conn.Close()

	frame := &EthernetFrame{
		Raw: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x08, 0x00},
	}

	err := conn.WriteFrame(frame)
	if err == nil {
		t.Errorf("Expected error when writing to closed connection")
	}

	if err.Error() != "connection closed" {
		t.Errorf("Expected 'connection closed' error, got: %v", err)
	}
}

func TestConnectionString(t *testing.T) {
	mockConn := &mockConn{
		addr: &mockAddr{network: "tcp", address: "127.0.0.1:8080"},
	}

	conn := NewConnection("test-conn", mockConn)

	str := conn.String()
	expected := "Connection[test-conn, remote=127.0.0.1:8080, frames_rx=0, frames_tx=0, closed=false]"

	if str != expected {
		t.Errorf("Expected string '%s', got '%s'", expected, str)
	}
}

func TestConnectionReadFrame(t *testing.T) {
	// Test successful read
	frameData := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x08, 0x00, 0x45, 0x00}
	lengthBytes := []byte{0x00, 0x00, 0x00, 0x10} // 16 bytes in big endian
	readData := append(lengthBytes, frameData...)

	mockConn := &mockConn{
		addr:     &mockAddr{network: "tcp", address: "127.0.0.1:8080"},
		readData: readData,
	}

	conn := NewConnection("test-conn", mockConn)

	frame, err := conn.ReadFrame()
	if err != nil {
		t.Errorf("Unexpected error reading frame: %v", err)
	}

	if frame == nil {
		t.Errorf("Expected frame to be returned")
		return
	}

	if len(frame.Raw) != len(frameData) {
		t.Errorf("Expected frame length %d, got %d", len(frameData), len(frame.Raw))
	}

	// Check statistics
	if conn.FramesReceived != 1 {
		t.Errorf("Expected 1 frame received, got %d", conn.FramesReceived)
	}

	if conn.BytesReceived != uint64(len(frameData)) {
		t.Errorf("Expected %d bytes received, got %d", len(frameData), conn.BytesReceived)
	}

	// Test read from closed connection
	conn.Close()
	_, err = conn.ReadFrame()
	if err == nil {
		t.Errorf("Expected error when reading from closed connection")
	}

	if err.Error() != "connection closed" {
		t.Errorf("Expected 'connection closed' error, got: %v", err)
	}
}

func TestConnectionReadFrameErrors(t *testing.T) {
	// Test short read for length
	mockConnShortLen := &mockConn{
		addr:     &mockAddr{network: "tcp", address: "127.0.0.1:8080"},
		readData: []byte{0x00, 0x00}, // Only 2 bytes instead of 4
	}

	conn := NewConnection("test-conn", mockConnShortLen)

	_, err := conn.ReadFrame()
	if err == nil {
		t.Errorf("Expected error when reading incomplete length header")
	}

	// Test frame too large
	mockConnLarge := &mockConn{
		addr:     &mockAddr{network: "tcp", address: "127.0.0.1:8080"},
		readData: []byte{0x00, 0x00, 0x20, 0x00}, // 8192 bytes - too large
	}

	conn2 := NewConnection("test-conn2", mockConnLarge)

	_, err = conn2.ReadFrame()
	if err == nil {
		t.Errorf("Expected error when frame is too large")
	}

	// Test short read for frame data
	frameLength := []byte{0x00, 0x00, 0x00, 0x10} // 16 bytes expected
	incompleteFrame := []byte{0x01, 0x02, 0x03}    // Only 3 bytes
	mockConnShort := &mockConn{
		addr:     &mockAddr{network: "tcp", address: "127.0.0.1:8080"},
		readData: append(frameLength, incompleteFrame...),
	}

	conn3 := NewConnection("test-conn3", mockConnShort)

	_, err = conn3.ReadFrame()
	if err == nil {
		t.Errorf("Expected error when reading incomplete frame data")
	}
}

func TestConnectionWriteFrameErrors(t *testing.T) {
	mockConn := &mockConn{
		addr: &mockAddr{network: "tcp", address: "127.0.0.1:8080"},
	}

	conn := NewConnection("test-conn", mockConn)

	// Test write with nil frame
	err := conn.WriteFrame(nil)
	if err == nil {
		t.Errorf("Expected error when writing nil frame")
	}

	// Test write with frame that has no raw data
	emptyFrame := &EthernetFrame{}
	err = conn.WriteFrame(emptyFrame)
	if err == nil {
		t.Errorf("Expected error when writing frame with no raw data")
	}
}

func TestConnectionRemoteAddr(t *testing.T) {
	mockConn := &mockConn{
		addr: &mockAddr{network: "tcp", address: "127.0.0.1:8080"},
	}

	conn := NewConnection("test-conn", mockConn)

	addr := conn.RemoteAddr()
	if addr != "127.0.0.1:8080" {
		t.Errorf("Expected remote address '127.0.0.1:8080', got '%s'", addr)
	}

	// Test with nil connection
	conn2 := &Connection{ID: "test", Conn: nil}
	addr2 := conn2.RemoteAddr()
	if addr2 != "unknown" {
		t.Errorf("Expected 'unknown' for nil connection, got '%s'", addr2)
	}
}
