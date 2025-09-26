package vswitch

import (
	"fmt"
	"net"
)

// EthernetFrame represents a parsed Ethernet frame
type EthernetFrame struct {
	DestMAC   net.HardwareAddr
	SrcMAC    net.HardwareAddr
	EtherType uint16
	Payload   []byte
	Raw       []byte
}

// BroadcastMAC is the Ethernet broadcast address
var BroadcastMAC = net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

// ParseEthernetFrame parses raw bytes into an EthernetFrame
func ParseEthernetFrame(data []byte) (*EthernetFrame, error) {
	if len(data) < 14 {
		return nil, fmt.Errorf("frame too short: %d bytes (minimum 14)", len(data))
	}

	frame := &EthernetFrame{
		Raw: make([]byte, len(data)),
	}
	copy(frame.Raw, data)

	// Parse Ethernet header (14 bytes)
	frame.DestMAC = net.HardwareAddr(data[0:6])
	frame.SrcMAC = net.HardwareAddr(data[6:12])
	frame.EtherType = uint16(data[12])<<8 | uint16(data[13])

	// Payload is everything after the 14-byte header
	frame.Payload = data[14:]

	return frame, nil
}

// IsBroadcast returns true if the frame is a broadcast frame
func (f *EthernetFrame) IsBroadcast() bool {
	return f.DestMAC.String() == BroadcastMAC.String()
}

// IsMulticast returns true if the frame is a multicast frame
func (f *EthernetFrame) IsMulticast() bool {
	return f.DestMAC[0]&0x01 == 1
}

// String returns a string representation of the frame
func (f *EthernetFrame) String() string {
	return fmt.Sprintf("Frame[%s -> %s, type=0x%04x, len=%d]",
		f.SrcMAC.String(), f.DestMAC.String(), f.EtherType, len(f.Raw))
}

// Validate performs basic frame validation
func (f *EthernetFrame) Validate() error {
	if len(f.Raw) < 14 {
		return fmt.Errorf("frame too short: %d bytes", len(f.Raw))
	}

	if len(f.Raw) > 1518 {
		return fmt.Errorf("frame too long: %d bytes", len(f.Raw))
	}

	// Check for valid MAC addresses (not all zeros)
	if f.SrcMAC.String() == "00:00:00:00:00:00" {
		return fmt.Errorf("invalid source MAC: all zeros")
	}

	return nil
}
