package vswitch

import (
	"net"
	"testing"
)

func TestParseEthernetFrame(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectError bool
		expectDest  string
		expectSrc   string
	}{
		{
			name:        "Valid frame",
			data:        []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x08, 0x00, 0x45, 0x00},
			expectError: false,
			expectDest:  "01:02:03:04:05:06",
			expectSrc:   "07:08:09:0a:0b:0c",
		},
		{
			name:        "Frame too short",
			data:        []byte{0x01, 0x02, 0x03},
			expectError: true,
		},
		{
			name:        "Minimum frame",
			data:        []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x08, 0x00},
			expectError: false,
			expectDest:  "01:02:03:04:05:06",
			expectSrc:   "07:08:09:0a:0b:0c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame, err := ParseEthernetFrame(tt.data)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
				return
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.expectError {
				return
			}

			if frame.DestMAC.String() != tt.expectDest {
				t.Errorf("Expected dest MAC %s, got %s", tt.expectDest, frame.DestMAC.String())
			}

			if frame.SrcMAC.String() != tt.expectSrc {
				t.Errorf("Expected src MAC %s, got %s", tt.expectSrc, frame.SrcMAC.String())
			}
		})
	}
}

func TestEthernetFrameIsBroadcast(t *testing.T) {
	tests := []struct {
		name           string
		destMAC        net.HardwareAddr
		expectedResult bool
	}{
		{
			name:           "Broadcast MAC",
			destMAC:        net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			expectedResult: true,
		},
		{
			name:           "Unicast MAC",
			destMAC:        net.HardwareAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := &EthernetFrame{
				DestMAC: tt.destMAC,
			}

			result := frame.IsBroadcast()
			if result != tt.expectedResult {
				t.Errorf("Expected %v, got %v", tt.expectedResult, result)
			}
		})
	}
}

func TestEthernetFrameIsMulticast(t *testing.T) {
	tests := []struct {
		name           string
		destMAC        net.HardwareAddr
		expectedResult bool
	}{
		{
			name:           "Multicast MAC (LSB set)",
			destMAC:        net.HardwareAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
			expectedResult: true,
		},
		{
			name:           "Unicast MAC (LSB not set)",
			destMAC:        net.HardwareAddr{0x02, 0x02, 0x03, 0x04, 0x05, 0x06},
			expectedResult: false,
		},
		{
			name:           "Broadcast MAC",
			destMAC:        net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := &EthernetFrame{
				DestMAC: tt.destMAC,
			}

			result := frame.IsMulticast()
			if result != tt.expectedResult {
				t.Errorf("Expected %v, got %v", tt.expectedResult, result)
			}
		})
	}
}

func TestEthernetFrameValidate(t *testing.T) {
	tests := []struct {
		name        string
		frame       *EthernetFrame
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid frame",
			frame: &EthernetFrame{
				Raw:    make([]byte, 64),
				SrcMAC: net.HardwareAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
			},
			expectError: false,
		},
		{
			name: "Frame too short",
			frame: &EthernetFrame{
				Raw: make([]byte, 10),
			},
			expectError: true,
			errorMsg:    "frame too short",
		},
		{
			name: "Frame too long",
			frame: &EthernetFrame{
				Raw: make([]byte, 2000),
			},
			expectError: true,
			errorMsg:    "frame too long",
		},
		{
			name: "Invalid source MAC (all zeros)",
			frame: &EthernetFrame{
				Raw:    make([]byte, 64),
				SrcMAC: net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			},
			expectError: true,
			errorMsg:    "invalid source MAC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.frame.Validate()

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
				return
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.expectError && tt.errorMsg != "" {
				if err.Error() != tt.errorMsg && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			}
		})
	}
}

func TestEthernetFrameString(t *testing.T) {
	frame := &EthernetFrame{
		DestMAC:   net.HardwareAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
		SrcMAC:    net.HardwareAddr{0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c},
		EtherType: 0x0800,
		Raw:       make([]byte, 64),
	}

	str := frame.String()
	expected := "Frame[07:08:09:0a:0b:0c -> 01:02:03:04:05:06, type=0x0800, len=64]"

	if str != expected {
		t.Errorf("Expected string '%s', got '%s'", expected, str)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(substr) <= len(s) && (substr == "" || s[len(s)-len(substr):] == substr ||
		   s[:len(substr)] == substr || (len(s) > len(substr) &&
		   s[1:len(substr)+1] == substr))
}
