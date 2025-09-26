// Package vswitch implements virtual Ethernet switching functionality.
package vswitch

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// Connection represents a single QEMU VM connection
type Connection struct {
	ID       string
	Conn     net.Conn
	LastSeen time.Time

	// Statistics
	FramesSent     uint64
	FramesReceived uint64
	BytesSent      uint64
	BytesReceived  uint64

	// Connection state
	mutex  sync.RWMutex
	closed bool
}

// NewConnection creates a new Connection instance
func NewConnection(id string, conn net.Conn) *Connection {
	return &Connection{
		ID:       id,
		Conn:     conn,
		LastSeen: time.Now(),
		closed:   false,
	}
}

// ReadFrame reads a single Ethernet frame from the connection
func (c *Connection) ReadFrame() (*EthernetFrame, error) {
	c.mutex.RLock()
	if c.closed {
		c.mutex.RUnlock()
		return nil, fmt.Errorf("connection closed")
	}
	c.mutex.RUnlock()

	// Read frame length (first 4 bytes in network byte order)
	lengthBytes := make([]byte, 4)
	if _, err := io.ReadFull(c.Conn, lengthBytes); err != nil {
		return nil, fmt.Errorf("failed to read frame length: %w", err)
	}

	// Convert to frame length (big endian)
	frameLen := uint32(lengthBytes[0])<<24 | uint32(lengthBytes[1])<<16 |
		uint32(lengthBytes[2])<<8 | uint32(lengthBytes[3])

	// Validate frame length
	if frameLen == 0 || frameLen > 1518 {
		return nil, fmt.Errorf("invalid frame length: %d", frameLen)
	}

	frameData := getFrameBuffer()[:frameLen]
	if _, err := io.ReadFull(c.Conn, frameData); err != nil {
		return nil, fmt.Errorf("failed to read frame data: %w", err)
	}

	// Parse the Ethernet frame
	frame, err := ParseEthernetFrame(frameData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse frame: %w", err)
	}

	// Validate the frame
	if err := frame.Validate(); err != nil {
		return nil, fmt.Errorf("invalid frame: %w", err)
	}

	// Update statistics
	c.mutex.Lock()
	c.FramesReceived++
	c.BytesReceived += uint64(len(frameData))
	c.LastSeen = time.Now()
	c.mutex.Unlock()

	return frame, nil
}

// WriteFrame writes an Ethernet frame to the connection
func (c *Connection) WriteFrame(frame *EthernetFrame) error {
	if frame == nil {
		return fmt.Errorf("frame cannot be nil")
	}

	c.mutex.RLock()
	if c.closed {
		c.mutex.RUnlock()
		return fmt.Errorf("connection closed")
	}
	c.mutex.RUnlock()

	if len(frame.Raw) == 0 {
		return fmt.Errorf("frame data cannot be empty")
	}

	frameData := frame.Raw
	dataLen := len(frameData)
	if dataLen > 0xFFFFFFFF {
		return fmt.Errorf("frame data too large: %d bytes", dataLen)
	}
	frameLen := uint32(dataLen)

	// Write frame length first (big endian)
	var lengthBytes [4]byte
	lengthBytes[0] = byte(frameLen >> 24)
	lengthBytes[1] = byte(frameLen >> 16)
	lengthBytes[2] = byte(frameLen >> 8)
	lengthBytes[3] = byte(frameLen)

	if _, err := c.Conn.Write(lengthBytes[:]); err != nil {
		return fmt.Errorf("failed to write frame length: %w", err)
	}

	// Write frame data
	if _, err := c.Conn.Write(frameData); err != nil {
		return fmt.Errorf("failed to write frame data: %w", err)
	}

	// Update statistics
	c.mutex.Lock()
	c.FramesSent++
	c.BytesSent += uint64(len(frameData))
	c.mutex.Unlock()

	return nil
}

// Close closes the connection
func (c *Connection) Close() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	if err := c.Conn.Close(); err != nil {
		log.Printf("Error closing connection %s: %v", c.ID, err)
		return err
	}

	log.Printf("Connection %s closed (sent: %d frames/%d bytes, received: %d frames/%d bytes)",
		c.ID, c.FramesSent, c.BytesSent, c.FramesReceived, c.BytesReceived)

	return nil
}

// IsClosed returns true if the connection is closed
func (c *Connection) IsClosed() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.closed
}

// RemoteAddr returns the remote address of the connection
func (c *Connection) RemoteAddr() string {
	if c.Conn != nil {
		return c.Conn.RemoteAddr().String()
	}
	return "unknown"
}

// String returns a string representation of the connection
func (c *Connection) String() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return fmt.Sprintf("Connection[%s, remote=%s, frames_rx=%d, frames_tx=%d, closed=%v]",
		c.ID, c.RemoteAddr(), c.FramesReceived, c.FramesSent, c.closed)
}
