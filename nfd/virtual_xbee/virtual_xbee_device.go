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

var ADDR_LIST = map[string]string{
	"0013a20041bb76a4": "localhost:8001",
	"0013a20041bb7684": "localhost:8002",
}

var peer = "0013a20041bb7684"

var child_ctx2, cancel2 = context.WithCancel(context.TODO())

type VirtualXbeeDevice struct {
	MACAddress string
	IPAddress  string
	Port       int
	Conn       *net.UDPConn
	RecvData   chan byte
	WG         sync.WaitGroup
}

func NewXbeeDevice(macAddress string, ipAddress string, port int) (*VirtualXbeeDevice, error) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", ipAddress, port))
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}

	device := &VirtualXbeeDevice{
		MACAddress: macAddress,
		IPAddress:  ipAddress,
		Port:       port,
		Conn:       conn,
		RecvData:   make(chan byte, 10000),
	}

	return device, nil
}

func (d *VirtualXbeeDevice) Write(data []byte, remote string) error {
	if remote != "" {
		remoteAddr, err := net.ResolveUDPAddr("udp", ADDR_LIST[remote])
		if ADDR_LIST[remote] == "" {
			remoteAddr, err = net.ResolveUDPAddr("udp", ADDR_LIST[peer])
		}
		if err != nil {
			return err
		}

		_, err = d.Conn.WriteToUDP(data, remoteAddr)
		if err != nil {
			fmt.Println(err)
			return err
		}
	}
	if data[FRAME_TYPE_OFFSET] == AT_COMMAND {
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
			prefix := []byte{0xFF, 0xFE}
			x64addr, _ := hex.DecodeString(peer)
			suffix := []byte{0x20, 0x00, 0xFF, 0xFE, 0x01, 0x00, 0xC1, 0x05, 0x10, 0x1E}
			body := append(append(prefix, x64addr...), suffix...)
			resp := ATCmdRespFrameToBytes(GenATCmdResp(AT_ND, data[FRAME_SEQ_OFFSET], body))
			for i := 0; i < len(resp); i++ {
				d.RecvData <- resp[i]
			}
		}
	} else if data[FRAME_TYPE_OFFSET] == TRANSMIT_REQUEST {
		resp := TransmitStatusFrameToBytes(GenTransmitStatus(data[FRAME_SEQ_OFFSET]))
		for i := 0; i < len(resp); i++ {
			d.RecvData <- resp[i]
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
