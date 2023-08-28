package xbee

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/tarm/serial"
)

const (
	DELIMITER               = 0x7e
	MAX_RECV_PACKET_CH_SIZE = 64
	MAX_RECV_RESP_CH_SIZE   = 64
	MAX_RECV_STATUS_CH_SIZE = 64
	MAX_RECV_BYTE_CH_SIZE   = 10000
)

var child_ctx, cancel = context.WithCancel(context.TODO())

type Reader struct {
	Port         serial.Port
	RecvRespCh   chan []byte
	RecvPacketCh chan []byte
	RecvByteCh   chan byte
	RecvStatusCh chan []byte
	started      bool

	Mutex sync.Mutex
	wg    sync.WaitGroup
}

func NewReader(port serial.Port) *Reader {
	return &Reader{
		Port:         port,
		RecvRespCh:   make(chan []byte, MAX_RECV_RESP_CH_SIZE),
		RecvPacketCh: make(chan []byte, MAX_RECV_PACKET_CH_SIZE),
		RecvStatusCh: make(chan []byte, MAX_RECV_STATUS_CH_SIZE),
		RecvByteCh:   make(chan byte, MAX_RECV_BYTE_CH_SIZE),
	}
}

func (reader *Reader) GetPacket(t int) ([]byte, error) {
	count := 0
	for {
		select {
		case temp := <-reader.RecvPacketCh:
			return temp, nil
		default:
			time.Sleep(10 * time.Millisecond)
			count++
			if count == t*100 {
				return nil, errors.New("get packet timeout")
			}
		}
	}
}

func (reader *Reader) GetResp(t int, seq byte) ([]byte, error) {
	count := 0
	for {
		select {
		case temp := <-reader.RecvRespCh:
			if temp[FRAME_SEQ_OFFSET] == seq {
				return temp, nil
			}
		default:
			time.Sleep(10 * time.Millisecond)
			count++
			if count == t*100 {
				return nil, errors.New("get response timeout")
			}
		}
	}
}

func (reader *Reader) GetStatus(t int, seq byte) ([]byte, error) {
	count := 0
	for {
		select {
		case temp := <-reader.RecvStatusCh:
			if temp[FRAME_SEQ_OFFSET] == seq {
				return temp, nil
			}
		default:
			time.Sleep(10 * time.Millisecond)
			count++
			if count == t*100 {
				return nil, errors.New("get status timeout")
			}
		}
	}
}

func (reader *Reader) ReadFrame(ctx context.Context) {
	defer reader.wg.Done()
	frame := make([]byte, 0)
	for {
		select {
		case b := <-reader.RecvByteCh:
			if b == DELIMITER {
				frame = append(frame, b)
				frame = append(frame, reader.ReadBytes(ctx, 2)...)
				frame = append(frame, reader.ReadBytes(ctx, int(frame[LEN_END_OFFSET])+1)...)
				temp := make([]byte, len(frame))
				copy(temp, frame)
				// if len(frame) == 29 {
				// 	logger.Infof(len(frame))
				// }
				// print(frame)
				if frame[FRAME_TYPE_OFFSET] == AT_COMMAND_RESPONSE {
					select {
					case reader.RecvRespCh <- temp:
					default:
						<-reader.RecvRespCh
						reader.RecvRespCh <- temp
					}
				} else if frame[FRAME_TYPE_OFFSET] == TRANSMIT_STATUS {
					select {
					case reader.RecvStatusCh <- temp:
					default:
						<-reader.RecvStatusCh
						reader.RecvStatusCh <- temp
					}
				} else if frame[FRAME_TYPE_OFFSET] == RECEIVED_PACKET {
					select {
					case reader.RecvPacketCh <- temp:
					default:
						<-reader.RecvPacketCh
						reader.RecvPacketCh <- temp
					}
				}
				// else if frame[FRAME_TYPE_OFFSET] == 0x8d {
				// 	print(frame)
				// }
				frame = make([]byte, 0)
			}
		case <-ctx.Done():
			return
		default:
		}
	}
}

func (reader *Reader) ReadBytes(ctx context.Context, count int) []byte {
	res := make([]byte, count)
	index := 0
	for index < count {
		select {
		case b := <-reader.RecvByteCh:
			res[index] = b
			index++
		case <-ctx.Done():
			return nil
		default:
		}
	}
	return res
}

func (reader *Reader) ReadByte(ctx context.Context) {
	defer reader.wg.Done()
	data := make([]byte, 1)
	for {
		_, err := reader.Port.Read(data)
		if err != nil {
			continue
		}
		select {
		case reader.RecvByteCh <- data[0]:
		case <-ctx.Done():
			return
		default:
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

func (reader *Reader) Start(ctx context.Context) {
	reader.started = true
	child_ctx, cancel = context.WithCancel(ctx)
	reader.wg.Add(2)
	go reader.ReadByte(child_ctx)
	go reader.ReadFrame(child_ctx)
}

func (reader *Reader) Stop() {
	reader.Clear()
	cancel()
	reader.wg.Wait()
	reader.started = false
}
