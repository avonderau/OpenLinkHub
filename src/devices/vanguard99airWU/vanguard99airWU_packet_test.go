package vanguard99airWU

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestWritePacketPreservesDeviceProtocolHeader(t *testing.T) {
	data := []byte{0xaa, 0xbb, 0xcc}
	prefix := []byte{0x12, 0x00}
	for _, tc := range []struct {
		name           string
		prefix         []byte
		countPrefix    bool
		expectedLength uint16
	}{
		{name: "key assignment", expectedLength: 3},
		{name: "color data", prefix: prefix, expectedLength: 3},
		{name: "actuation data", prefix: prefix, countPrefix: true, expectedLength: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			packet, err := buildVanguardWritePacket(tc.prefix, data, tc.countPrefix)
			if err != nil {
				t.Fatal(err)
			}
			if len(packet) != headerWriteSize+len(tc.prefix)+len(data) {
				t.Fatalf("incorrect packet size: %d", len(packet))
			}
			if binary.LittleEndian.Uint16(packet[:2]) != tc.expectedLength {
				t.Fatalf("incorrect length header: %d", binary.LittleEndian.Uint16(packet[:2]))
			}
			if !bytes.Equal(packet[headerWriteSize:headerWriteSize+len(tc.prefix)], tc.prefix) {
				t.Fatal("packet prefix changed")
			}
			if !bytes.Equal(packet[headerWriteSize+len(tc.prefix):], data) {
				t.Fatal("packet payload changed")
			}
		})
	}
}

func TestWritePacketRejectsOversizePayloadWithoutAllocating(t *testing.T) {
	const maxPayload = int(^uint16(0))
	for _, tc := range []struct {
		name   string
		prefix []byte
		data   []byte
	}{
		{name: "payload cannot fit length field", data: make([]byte, maxPayload+1)},
		{name: "prefix and payload cannot fit together", prefix: []byte{0x12, 0x00}, data: make([]byte, maxPayload-1)},
		{name: "oversized prefix", prefix: make([]byte, maxPayload+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			packet, err := buildVanguardWritePacket(tc.prefix, tc.data, true)
			if err == nil || packet != nil {
				t.Fatalf("oversize packet accepted: len=%d err=%v", len(packet), err)
			}
		})
	}

	data := make([]byte, maxPayload-2)
	packet, err := buildVanguardWritePacket([]byte{0x30, 0x00}, data, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(packet) != headerWriteSize+maxPayload ||
		binary.LittleEndian.Uint16(packet[:2]) != uint16(maxPayload) {
		t.Fatal("maximum safe packet size was not encoded correctly")
	}
}
