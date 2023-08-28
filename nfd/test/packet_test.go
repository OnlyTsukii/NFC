package nfd_test

import (
	"bytes"

	"gitee.com/ccl0924/nfd/nfd"

	"testing"
)

func TestPacketEncodingAndDecoding(t *testing.T) {
	seq := 123
	packetType := 1
	srcMAC := "0011223344556677"
	destMAC := "8899aabbccddeeff"
	data := []byte{0x01, 0x02, 0x03, 0x04}

	originalPacket := nfd.NewPacket(seq, packetType, srcMAC, destMAC, data)
	encodedData, _ := originalPacket.Encode()

	decodedPacket, err := nfd.DecodePacket(encodedData)
	if err != nil {
		t.Fatalf("Error decoding packet: %v", err)
	}

	if decodedPacket.Seq != seq ||
		decodedPacket.PacketType != packetType ||
		decodedPacket.SrcMac != srcMAC ||
		decodedPacket.DestMac != destMAC ||
		!bytes.Equal(decodedPacket.Data, data) {
		t.Errorf("Decoded packet does not match the original packet")
	}
}

func TestPacketStringRepresentation(t *testing.T) {
	packet := &nfd.Packet{
		Seq:        123,
		PacketType: 1,
		SrcMac:     "0011223344556677",
		DestMac:    "8899aabbccddeeff",
		Data:       []byte{0x01, 0x02, 0x03, 0x04},
	}

	expectedString := "Packet(seq=123, packetType=1, srcMAC=0011223344556677, destMAC=8899aabbccddeeff, len(data)=4)"
	if packet.String() != expectedString {
		t.Errorf("String representation of packet is incorrect")
	}
}
