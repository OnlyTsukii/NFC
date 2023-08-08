package xbee

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tarm/serial"
)

const (
	AT_COMMAND          = 0x08
	AT_COMMAND_RESPONSE = 0x88
	TRANSMIT_REQUEST    = 0x10
	TRANSMIT_STATUS     = 0x8B
	RECEIVED_PACKET     = 0x90

	MAX_PACKET_LENGTH = 256

	DELIMITER_OFFSET  = 0
	LEN_START_OFFSET  = 1
	LEN_END_OFFSET    = 2
	FRAME_TYPE_OFFSET = 3
	FRAME_SEQ_OFFSET  = 4

	AT_RESP_STATUS_OFFSET   = 7
	AT_RESP_RESPONSE_OFFSET = 8

	TRANSMIT_STATUS_OFFSET = 8

	API_REQ_X64ADDR_OFFSET = 5
	API_REQ_X16ADDR_OFFSET = 13
	API_REQ_BR_OFFSET      = 15
	API_REQ_OPTIONS_OFFSET = 16

	API_RECV_X64ADDR_OFFSET = 4
	API_RECV_X16ADDR_OFFSET = 12
	API_RECV_DATA_OFFSET    = 15

	BCST_X64_ADDR    = "000000000000FFFF"
	RESPONSE_TIMEOUT = 5
)

type Data struct {
	Fragments []*Fragment
}

func NewData() *Data {
	return &Data{
		Fragments: []*Fragment{},
	}
}

func (d *Data) Append(frag *Fragment) {
	d.Fragments = append(d.Fragments, frag)
}

func (d *Data) GetData() []byte {
	var data []byte
	for _, frag := range d.Fragments {
		data = append(data, frag.Data...)
	}
	return data
}

type Xbee struct {
	Port    serial.Port
	Mac     string
	Seq     int
	Reader  *Reader
	Writer  *Writer
	Started bool

	Mutex sync.Mutex
}

func NewXbee(port string, baudrate int) (*Xbee, error) {
	x := Xbee{Seq: -1}
	conf := &serial.Config{Name: port, Baud: baudrate, ReadTimeout: 500 * time.Millisecond}
	p, err := serial.OpenPort(conf)
	if err != nil {
		return nil, err
	}
	x.Port = *p
	x.Writer = NewWriter(*p)
	x.Reader = NewReader(*p)
	err = x.SetMacAddr()
	if err != nil {
		return nil, err
	}
	return &x, nil
}

func (x *Xbee) SetMacAddr() error {
	ctx, _ := context.WithCancel(context.TODO())
	if !x.Reader.started {
		x.Reader.Start(ctx)
	}
	var mac []string
	ATs := []string{"SH", "SL"}
	res := make(map[string][]byte, 0)
	err := errors.New("")
	for i := 0; i < 3; i++ {
		res, err = x.Writer.SendATCmdWithResponse(ATs, x.Reader)
		if err == nil {
			break
		}
	}
	if err != nil {
		if x.Reader.started {
			x.Reader.Stop()
		}
		return err
	}
	for _, v := range ATs {
		for _, v := range res[v] {
			mac = append(mac, fmt.Sprintf("%02x", v))
		}
	}
	x.Mac = strings.Join(mac, "")
	if x.Reader.started {
		x.Reader.Stop()
	}
	return nil
}

func (x *Xbee) GetNodes() ([]string, error) {
	addrs := make([]string, 0)
	resp, err := x.Writer.GetNodes(x.Reader)
	if err != nil {
		return nil, err
	}
	for _, v := range resp {
		addrs = append(addrs, BytesToStr(v))
	}
	return addrs, nil
}

func (x *Xbee) SendData(x64addr string, data []byte) error {
	return x.Writer.SendPacketWithResponse(x64addr, data, x.Reader)
}

func (x *Xbee) RecvData() ([]byte, string, error) {
	packet, err := x.Reader.GetPacket(RESPONSE_TIMEOUT)
	if err != nil || packet == nil {
		return nil, "", err
	}
	if Check(packet) {
		return packet[API_RECV_DATA_OFFSET : len(packet)-1],
			BytesToStr(packet[API_RECV_X64ADDR_OFFSET:API_RECV_X16ADDR_OFFSET]), nil
	} else {
		return nil, "", errors.New("received packet is corrupted")
	}
}

func (x *Xbee) SendPacket(data []byte, remoteAddr string) {
	x.Mutex.Lock()
	defer x.Mutex.Unlock()

	x.Seq = (x.Seq + 1) % 256
	fragments := GetFragments(x.Seq, data)
	for _, frag := range fragments {
		err := x.SendData(remoteAddr, frag.Encode())
		if err != nil {
			fmt.Println("xbee failed to send data")
			break
		}
	}
}

func (x *Xbee) ReceivePacket() ([]byte, error) {
	receivedData := make(map[string]*Data)
	var originalMessage []byte
	for x.Started {
		fragment, remoteAddr, err := x.RecvData()
		if err != nil {
			return nil, err
		}
		frag, err := DecodeFragment(fragment)
		if err != nil {
			return nil, err
		}
		index := fmt.Sprintf("%s:%d", remoteAddr, frag.No)
		if _, ok := receivedData[index]; !ok {
			receivedData[index] = NewData()
		}
		receivedData[index].Append(frag)
		if len(receivedData[index].Fragments) == frag.Total {
			originalMessage = receivedData[index].GetData()
			for key := range receivedData {
				delete(receivedData, key)
			}
			return originalMessage, nil
		}
	}
	return originalMessage, nil
}

func (x *Xbee) Start(ctx context.Context) {
	x.Started = true
	if !x.Reader.started {
		x.Reader.Start(ctx)
	}
}

func (x *Xbee) Stop() {
	x.Started = false
	if x.Reader.started {
		x.Reader.Stop()
	}
}

func (x *Xbee) Close() {
	x.Started = false
	if x.Reader.started {
		x.Reader.Stop()
	}
	x.Port.Close()
}
