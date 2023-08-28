package vxbee

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
)

var (
	AT_SH = []byte{0x53, 0x48}
	AT_SL = []byte{0x53, 0x4C}
	AT_ND = []byte{0x4E, 0x44}
)

var child_ctx2, cancel2 = context.WithCancel(context.TODO())

type VirtualXbeeDevice struct {
	MACAddress string
	IPAddress  string
	Port       int
	Conn       *net.UDPConn
	RecvData   chan byte
	WG         sync.WaitGroup
	AddrMap    map[string]string
}

func NewXbeeDevice(macAddress string, AddrMap map[string]string, UDPPort int) (*VirtualXbeeDevice, error) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", "0.0.0.0", UDPPort))
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}

	device := &VirtualXbeeDevice{
		MACAddress: macAddress,
		Conn:       conn,
		RecvData:   make(chan byte, 10000),
		AddrMap:    AddrMap,
	}

	return device, nil
}

func (d *VirtualXbeeDevice) Write(data []byte, remote string) error {
	if remote != "" {
		if d.AddrMap[remote] == "" {
			for key := range d.AddrMap {
				if key != d.MACAddress {
					remoteAddr, err := net.ResolveUDPAddr("udp", d.AddrMap[key])
					if err != nil {
						return err
					}

					_, err = d.Conn.WriteToUDP(data, remoteAddr)
					if err != nil {
						return err
					}

					resp := TransmitStatusFrameToBytes(GenTransmitStatus(data[FRAME_SEQ_OFFSET]))
					for i := 0; i < len(resp); i++ {
						d.RecvData <- resp[i]
					}
				}
			}
		} else {
			remoteAddr, err := net.ResolveUDPAddr("udp", d.AddrMap[remote])
			if err != nil {
				return err
			}

			_, err = d.Conn.WriteToUDP(data, remoteAddr)
			if err != nil {
				return err
			}

			resp := TransmitStatusFrameToBytes(GenTransmitStatus(data[FRAME_SEQ_OFFSET]))
			for i := 0; i < len(resp); i++ {
				d.RecvData <- resp[i]
			}
		}
	} else {
		if data[AT_COMMAND_OFFSET] == AT_SH[0] && data[AT_COMMAND_OFFSET+1] == AT_SH[1] {
			SH, _ := hex.DecodeString(d.MACAddress[:8])
			resp := ATCmdRespFrameToBytes(GenATCmdResp(AT_SH, data[FRAME_SEQ_OFFSET], SH))
			for i := 0; i < len(resp); i++ {
				d.RecvData <- resp[i]
			}
		} else if data[AT_COMMAND_OFFSET] == AT_SL[0] && data[AT_COMMAND_OFFSET+1] == AT_SL[1] {
			SL, _ := hex.DecodeString(d.MACAddress[8:])
			resp := ATCmdRespFrameToBytes(GenATCmdResp(AT_SL, data[FRAME_SEQ_OFFSET], SL))
			for i := 0; i < len(resp); i++ {
				d.RecvData <- resp[i]
			}
		} else if data[AT_COMMAND_OFFSET] == AT_ND[0] && data[AT_COMMAND_OFFSET+1] == AT_ND[1] {
			for key := range d.AddrMap {
				if key != d.MACAddress {
					prefix := []byte{0xFF, 0xFE}
					x64addr, _ := hex.DecodeString(key)
					suffix := []byte{0x20, 0x00, 0xFF, 0xFE, 0x01, 0x00, 0xC1, 0x05, 0x10, 0x1E}
					body := append(append(prefix, x64addr...), suffix...)
					resp := ATCmdRespFrameToBytes(GenATCmdResp(AT_ND, data[FRAME_SEQ_OFFSET], body))
					for i := 0; i < len(resp); i++ {
						d.RecvData <- resp[i]
					}
				}
			}
		}
	}
	return nil
}

func (d *VirtualXbeeDevice) Read(ctx context.Context) {
	defer d.WG.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			buffer := make([]byte, 256)
			n, _, err := d.Conn.ReadFromUDP(buffer)
			if err != nil {
				continue
			}
			addr := buffer[API_REQ_X64ADDR_OFFSET : API_REQ_X64ADDR_OFFSET+8]
			data := buffer[API_REQ_DATA_OFFSET : n-1]
			recv := ReceivePacketFrameToBytes(GenReceivePacket(addr, data))
			for i := 0; i < len(recv); i++ {
				d.RecvData <- recv[i]
			}
		}
	}
}

func (d *VirtualXbeeDevice) Close() {
	d.Conn.Close()
}

func (d *VirtualXbeeDevice) Start(ctx context.Context) {
	child_ctx2, cancel2 = context.WithCancel(ctx)
	d.WG.Add(1)
	go d.Read(child_ctx2)
}

func (d *VirtualXbeeDevice) Stop() {
	d.Close()
	cancel2()
	d.WG.Wait()
}
