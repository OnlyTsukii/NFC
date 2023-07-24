package xbee

import (
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

type Xbee struct {
	Port    serial.Port
	Mac     string
	Seq     int
	Reader  *Reader
	Sender  *Sender
	Started bool
	SeqMap  map[string][]int
	Mutex   sync.Mutex
}

func NewXbee(port string, baudrate int) (*Xbee, error) {
	x := Xbee{Seq: -1}
	conf := &serial.Config{Name: port, Baud: baudrate}
	p, err := serial.OpenPort(conf)
	if err != nil {
		return nil, errors.New("open serial port failed")
	}
	x.Port = *p
	x.SeqMap = make(map[string][]int)
	x.Sender = NewSender(*p)
	x.Reader = NewReader(*p)
	x.SetMacAddr()
	return &x, nil
}

func (x *Xbee) SetMacAddr() error {
	x.Reader.Start()
	var mac []string
	ATs := []string{"SH", "SL"}
	resp, err := x.Sender.SendATCmdWithResponse(ATs, x.Reader)
	if err != nil {
		return err
	}
	for _, v := range ATs {
		for _, v := range resp[v] {
			mac = append(mac, fmt.Sprintf("%02x", v))
		}
	}
	x.Mac = strings.Join(mac, "")
	x.Reader.Stop()
	return nil
}

func (x *Xbee) SendData(x64addr string, data []byte) error {
	return x.Sender.SendPacketWithResponse(x64addr, data, x.Reader)
}

func (x *Xbee) RecvData() ([]byte, string, error) {
	packet, err := x.Reader.GetPacket(RESPONSE_TIMEOUT)
	if err != nil || packet == nil {
		return nil, "", err
	}
	if Check(packet) {
		// fmt.Println(len(packet))
		return packet[API_RECV_DATA_OFFSET : len(packet)-1],
			BytesToStr(packet[API_RECV_X64ADDR_OFFSET:API_RECV_X16ADDR_OFFSET]), nil
	} else {
		return nil, "", errors.New("received packet is corrupted")
	}
}

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

func (x *Xbee) SendPacket(data []byte, remoteAddr string) bool {
	x.Mutex.Lock()
	defer x.Mutex.Unlock()
	x.Seq = (x.Seq + 1) % 256
	fragments := GetFragments(x.Seq, data)
	// fmt.Println(len(fragments))
	// cur := time.Now()
	for _, frag := range fragments {
		count := 0
		for x.Started {
			err := x.SendData(remoteAddr, frag.Encode())
			count++
			if err == nil {
				break
			} else if count == 5 {
				return false
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
	// fmt.Println(time.Since(cur))
	return true
}

func (x *Xbee) ReceivePacket() ([]byte, error) {
	receivedData := make(map[string]*Data)
	var originalMessage []byte
	for x.Started {
		fragment, remoteAddr, err := x.RecvData()
		if err != nil {
			continue
		}
		frag, err := DecodeFragment(fragment)
		if err != nil {
			continue
		}
		if kv, ok := x.SeqMap[remoteAddr]; !ok {
			kv := make([]int, 2)
			kv[0] = frag.No
			kv[1] = (kv[1] + 1) % frag.Total
			if kv[1] == 0 {
				kv[0]++
			}
			x.SeqMap[remoteAddr] = kv
		} else if kv[0] == frag.No {
			if kv[1] == frag.Seq {
				kv[1] = (kv[1] + 1) % frag.Total
				if kv[1] == 0 {
					kv[0]++
				}
			} else if kv[1] < frag.Seq {
				return nil, errors.New("received a disorder fragment")
			} else if kv[1] > frag.Seq {
				continue
			}
		}
		index := fmt.Sprintf("%s:%d", remoteAddr, frag.No)
		if _, ok := receivedData[index]; !ok {
			receivedData[index] = NewData()
		}
		// fmt.Printf("%s:Xbee receive packet: no %d seq %d len %d \n", x.Mac, frag.No, frag.Seq, len(frag.Data))
		receivedData[index].Append(frag)
		if len(receivedData[index].Fragments) == frag.Total {
			originalMessage = receivedData[index].GetData()
			delete(receivedData, index)
			break
		}
	}
	return originalMessage, nil
}

func (x *Xbee) Start() {
	x.Started = true
	x.Reader.Start()
}

func (x *Xbee) Stop() {
	x.Started = false
	x.Reader.Stop()
}

func (x *Xbee) Close() {
	x.Stop()
	x.Port.Close()
}
