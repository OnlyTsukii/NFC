package nfc

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	HEADSIZE = 19

	SEQ_OFFSET         = 0
	PACKET_TYPE_OFFSET = 2
	SRC_MAC_OFFSET     = 3
	DEST_MAC_OFFSET    = 11
	DATA_OFFSET        = 19
)

type Packet struct {
	Seq        int
	PacketType int
	SrcMac     string
	DestMac    string
	Data       []byte
}

func NewPacket(seq, packetType int, srcMAC, destMAC string, data []byte) *Packet {
	return &Packet{
		Seq:        seq,
		PacketType: packetType,
		SrcMac:     srcMAC,
		DestMac:    destMAC,
		Data:       data,
	}
}

func (p *Packet) Encode() []byte {
	srcMAC := MacToHex(p.SrcMac)
	destMAC := MacToHex(p.DestMac)
	data := p.Data

	packetData := make([]byte, HEADSIZE+len(data))
	binary.BigEndian.PutUint16(packetData[SEQ_OFFSET:PACKET_TYPE_OFFSET], uint16(p.Seq))
	packetData[PACKET_TYPE_OFFSET] = byte(p.PacketType)
	copy(packetData[SRC_MAC_OFFSET:DEST_MAC_OFFSET], srcMAC)
	copy(packetData[DEST_MAC_OFFSET:DATA_OFFSET], destMAC)
	copy(packetData[DATA_OFFSET:], data)

	return packetData
}

func DecodePacket(data []byte) (*Packet, error) {
	if len(data) < HEADSIZE+1 {
		return nil, errors.New("data too small")
	}
	seq := int(binary.BigEndian.Uint16(data[SEQ_OFFSET:PACKET_TYPE_OFFSET]))
	packetType := int(data[PACKET_TYPE_OFFSET])
	srcMAC := HexToMac(data[SRC_MAC_OFFSET:DEST_MAC_OFFSET])
	destMAC := HexToMac(data[DEST_MAC_OFFSET:DATA_OFFSET])
	new_data := data[DATA_OFFSET:]

	return &Packet{
		Seq:        seq,
		PacketType: packetType,
		SrcMac:     srcMAC,
		DestMac:    destMAC,
		Data:       new_data,
	}, nil
}

func (p *Packet) String() string {
	return fmt.Sprintf("Packet(seq=%d, packetType=%d, srcMAC=%s, destMAC=%s, len(data)=%v)",
		p.Seq, p.PacketType, p.SrcMac, p.DestMac, len(p.Data))
}
