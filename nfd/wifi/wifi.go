package wifi

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
	SRC_MAC_OFFSET          = 6
	DST_MAC_OFFSET          = 11
	RECEIVED_PAYLOAD_OFFSET = 16
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

type Wifi struct {
	Port    serial.Port
	MAC     string
	Seq     int
	Reader  *Reader
	Writer  *Writer
	Started bool

	Mutex sync.Mutex
}

func NewWifi(port string, baudrate int) (*Wifi, error) {
	w := Wifi{Seq: -1}
	conf := &serial.Config{Name: port, Baud: baudrate, ReadTimeout: 500 * time.Millisecond}
	p, err := serial.OpenPort(conf)
	if err != nil {
		return nil, err
	}
	w.Port = *p
	w.Writer = NewWriter(p)
	w.Reader = NewReader(p)
	err = w.SetMacAddr()
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (w *Wifi) SetMacAddr() error {
	ctx, _ := context.WithCancel(context.TODO())
	if !w.Reader.started {
		w.Reader.Start(ctx)
	}
	var mac []string
	res := make([]byte, 0)
	err := errors.New("")
	for i := 0; i < 3; i++ {
		res, err = w.Writer.SendAddrReqWithResponse(w.Reader)
		if err == nil {
			break
		}
	}
	if err != nil {
		if w.Reader.started {
			w.Reader.Stop()
		}
		return err
	}
	for _, v := range res {
		mac = append(mac, fmt.Sprintf("%02x", v))
	}
	w.MAC = strings.Join(mac, "")
	if w.Reader.started {
		w.Reader.Stop()
	}
	return nil
}

func (w *Wifi) GetNodes() ([]string, error) {
	addrs := make([]string, 0)
	resp, err := w.Writer.GetNodes(w.Reader)
	if err != nil {
		return nil, err
	}
	for _, v := range resp {
		addrs = append(addrs, BytesToStr(v))
	}
	return addrs, nil
}

func (w *Wifi) SendData(dest string, data []byte) error {
	return w.Writer.SendPacketWithResponse(w.MAC, dest, data, w.Reader)
}

func (w *Wifi) RecvData() ([]byte, string, error) {
	packet, err := w.Reader.GetPacket(RESPONSE_TIMEOUT)
	if err != nil || packet == nil {
		return nil, "", err
	}
	if validate(packet) {
		return packet[RECEIVED_PAYLOAD_OFFSET : len(packet)-1],
			BytesToStr(packet[DST_MAC_OFFSET:RECEIVED_PAYLOAD_OFFSET]), nil
	} else {
		return nil, "", errors.New("received packet is corrupted")
	}
}

func (w *Wifi) SendPacket(data []byte, remoteAddr string) bool {
	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	remoteAddr = remoteAddr[:10]

	w.Seq = (w.Seq + 1) % 256
	fragments, err := GetFragments(w.Seq, data)
	if err != nil {
		return false
	}
	for _, frag := range fragments {
		err := w.SendData(remoteAddr, frag.Encode())
		if err != nil {
			return false
		}
	}
	return true
}

func (w *Wifi) ReceivePacket() ([]byte, error) {
	receivedData := make(map[string]*Data)
	var originalMessage []byte
	for w.Started {
		fragment, remoteAddr, err := w.RecvData()
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

func (w *Wifi) Start(ctx context.Context) {
	w.Started = true
	if !w.Reader.started {
		w.Reader.Start(ctx)
	}
}

func (w *Wifi) Stop() {
	w.Started = false
	if w.Reader.started {
		w.Reader.Stop()
	}
}

func (w *Wifi) Close() {
	w.Started = false
	if w.Reader.started {
		w.Reader.Stop()
	}
	w.Port.Close()
}
