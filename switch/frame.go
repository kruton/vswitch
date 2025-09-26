package vswitch

import (
	"fmt"
	"net"
)

// EthernetFrame represents a parsed Ethernet frame
type EthernetFrame struct {
	Raw       []byte
	DestMAC   net.HardwareAddr
	SrcMAC    net.HardwareAddr
	EtherType uint16
	Payload   []byte
	pooled    bool
}

// BroadcastMAC is the Ethernet broadcast address
var BroadcastMAC = net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

// ParseEthernetFrame parses raw bytes into an EthernetFrame
func ParseEthernetFrame(data []byte) (*EthernetFrame, error) {
	if len(data) < 14 {
		return nil, fmt.Errorf("frame too short: %d bytes (minimum 14)", len(data))
	}

	frame := &EthernetFrame{
		Raw:       data,
		DestMAC:   data[0:6],
		SrcMAC:    data[6:12],
		EtherType: uint16(data[12])<<8 | uint16(data[13]),
		Payload:   data[14:],
		pooled:    true,
	}

	return frame, nil
}

// Release returns the frame buffer to the pool if it was pooled
func (f *EthernetFrame) Release() {
	if f.pooled && f.Raw != nil {
		putFrameBuffer(f.Raw)
		f.Raw = nil
		f.pooled = false
	}
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
