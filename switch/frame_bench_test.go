package vswitch

import (
	"testing"
)

var testFrameData = []byte{
	0x52, 0x54, 0x00, 0x12, 0x34, 0x56, // Dest MAC
	0x52, 0x54, 0x00, 0x78, 0x9a, 0xbc, // Src MAC
	0x08, 0x00, // EtherType (IPv4)
	0x45, 0x00, 0x00, 0x54, 0x12, 0x34, 0x40, 0x00, 0x40, 0x01, // IPv4 header
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Padding
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

func BenchmarkParseEthernetFrame(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frame, err := ParseEthernetFrame(testFrameData)
		if err != nil {
			b.Fatal(err)
		}
		frame.Release()
	}
}

func BenchmarkParseEthernetFrameWithPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate reading from network with pooled buffer
		buf := getFrameBuffer()[:len(testFrameData)]
		copy(buf, testFrameData)

		frame, err := ParseEthernetFrame(buf)
		if err != nil {
			b.Fatal(err)
		}
		frame.Release()
	}
}

func BenchmarkFrameValidation(b *testing.B) {
	frame, err := ParseEthernetFrame(testFrameData)
	if err != nil {
		b.Fatal(err)
	}
	defer frame.Release()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = frame.Validate()
	}
}

func BenchmarkMACComparison(b *testing.B) {
	frame, err := ParseEthernetFrame(testFrameData)
	if err != nil {
		b.Fatal(err)
	}
	defer frame.Release()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = frame.IsBroadcast()
		_ = frame.IsMulticast()
	}
}
