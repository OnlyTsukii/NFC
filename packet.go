package main

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type Packet struct {
	Seq        int
	PacketType int
	SrcMac     string
	DestMac    string
	SrcIP      string
	DestIP     string
	Data       []byte
}

func NewPacket(seq, packetType int, srcMAC, destMAC, srcIP, destIP string, data []byte) *Packet {
	return &Packet{
		Seq:        seq,
		PacketType: packetType,
		SrcMac:     srcMAC,
		DestMac:    destMAC,
		SrcIP:      srcIP,
		DestIP:     destIP,
		Data:       data,
	}
}

func (p *Packet) Encode() []byte {
	srcIP := ip2hex(p.SrcIP)
	destIP := ip2hex(p.DestIP)
	srcMAC := []byte(p.SrcMac)
	destMAC := []byte(p.DestMac)
	data := p.Data

	packetData := make([]byte, 51+len(data))
	binary.BigEndian.PutUint16(packetData[0:2], uint16(p.Seq))
	packetData[2] = byte(p.PacketType)
	copy(packetData[3:19], srcMAC)
	copy(packetData[19:35], destMAC)
	copy(packetData[35:43], srcIP)
	copy(packetData[43:51], destIP)
	copy(packetData[51:], data)

	return packetData
}

func DecodePacket(data []byte) (*Packet, error) {
	if len(data) < 51 {
		return nil, errors.New("data too small")
	}
	seq := int(binary.BigEndian.Uint16(data[0:2]))
	packetType := int(data[2])
	srcMAC := string(data[3:19])
	destMAC := string(data[19:35])
	srcIP := hex2ip(string(data[35:43]))
	destIP := hex2ip(string(data[43:51]))
	new_data := data[51:]

	return &Packet{
		Seq:        seq,
		PacketType: packetType,
		SrcMac:     srcMAC,
		DestMac:    destMAC,
		SrcIP:      srcIP,
		DestIP:     destIP,
		Data:       new_data,
	}, nil
}

func (p *Packet) String() string {
	return fmt.Sprintf("Packet(seq=%d, packetType=%d, srcMAC=%s, destMAC=%s, srcIP=%s, destIP=%s, len(data)=%v)",
		p.Seq, p.PacketType, p.SrcMac, p.DestMac, p.SrcIP, p.DestIP, len(p.Data))
}
