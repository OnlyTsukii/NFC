package main

import (
	"errors"
	"sync"
	"time"

	"github.com/tarm/serial"
)

const (
	DELIMITER          = 0x7e
	MAX_RECV_PACKET_CH = 40
	MAX_RECV_RESP_CH   = 40
	MAX_RECV_BYTE_CH   = 10000
)

var STOP_CMD = []byte{0x7e, 0x00, 0x04, 0x09, 0x01, 0x41, 0x50, 0x64}

type Reader struct {
	Port         serial.Port
	RecvRespCh   chan []byte
	RecvPacketCh chan []byte
	RecvByteCh   chan byte
	Started      bool
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

func NewReader(port serial.Port) *Reader {
	return &Reader{
		Port:         port,
		RecvRespCh:   make(chan []byte, MAX_RECV_RESP_CH),
		RecvPacketCh: make(chan []byte, MAX_RECV_PACKET_CH),
		RecvByteCh:   make(chan byte, MAX_RECV_BYTE_CH),
		Started:      false,
		stopCh:       make(chan struct{}),
		wg:           sync.WaitGroup{},
	}
}

func makeTimeout(ch chan bool, t int) {
	time.Sleep(time.Second * time.Duration(t))
	ch <- true
}

func (reader *Reader) GetPacket(t int) ([]byte, error) {
	timeout := make(chan bool, 1)
	go makeTimeout(timeout, t)
	select {
	case temp := <-reader.RecvPacketCh:
		return temp, nil
	case <-timeout:
		return nil, errors.New("get packet timeout")
	case <-reader.stopCh:
		return nil, nil
	}
}

func (reader *Reader) GetResp(t int) ([]byte, error) {
	timeout := make(chan bool, 1)
	go makeTimeout(timeout, t)
	select {
	case temp := <-reader.RecvRespCh:
		return temp, nil
	case <-timeout:
		return nil, errors.New("get response timeout")
	case <-reader.stopCh:
		return nil, nil
	}
}

func (reader *Reader) ReadFrame() {
	defer reader.wg.Done()
	frame := make([]byte, 0)
	for {
		select {
		case b := <-reader.RecvByteCh:
			if b == DELIMITER {
				frame = append(frame, b)
				frame = append(frame, reader.ReadBytes(2)...)
				frame = append(frame, reader.ReadBytes(int(frame[LEN_END_OFFSET])+1)...)
				temp := make([]byte, len(frame))
				copy(temp, frame)
				// print(frame)
				if frame[FRAME_TYPE_OFFSET] == AT_COMMAND_RESPONSE || frame[FRAME_TYPE_OFFSET] == TRANSMIT_STATUS {
					select {
					case reader.RecvRespCh <- temp:
					default:
						<-reader.RecvRespCh
						reader.RecvRespCh <- temp
					}
				} else if frame[FRAME_TYPE_OFFSET] == RECEIVED_PACKET {
					select {
					case reader.RecvPacketCh <- temp:
					default:
						<-reader.RecvPacketCh
						reader.RecvPacketCh <- temp
					}
				}
				frame = make([]byte, 0)
			}
		case <-reader.stopCh:
			return
		}
	}
}

func (reader *Reader) ReadBytes(count int) []byte {
	res := make([]byte, count)
	index := 0
	for index < count {
		select {
		case b := <-reader.RecvByteCh:
			res[index] = b
			index++
		case <-reader.stopCh:
			return nil
		}
	}
	return res
}

func (reader *Reader) ReadByte() {
	defer reader.wg.Done()
	for {
		data := make([]byte, 1)
		reader.Port.Read(data)
		select {
		case reader.RecvByteCh <- data[0]:
		case <-reader.stopCh:
			return
		}
	}
}

func (reader *Reader) Clear() {
	for len(reader.RecvByteCh) > 0 {
		<-reader.RecvByteCh
	}
	for len(reader.RecvRespCh) > 0 {
		<-reader.RecvRespCh
	}
	for len(reader.RecvPacketCh) > 0 {
		<-reader.RecvPacketCh
	}
}

func (reader *Reader) Start() {
	reader.stopCh = make(chan struct{})
	reader.wg.Add(2)
	go reader.ReadByte()
	go reader.ReadFrame()
}

func (reader *Reader) Stop() {
	close(reader.stopCh)
	reader.Port.Write(STOP_CMD)
	reader.wg.Wait()
	reader.Clear()
}
