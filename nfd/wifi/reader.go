package wifi

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tarm/serial"
)

const (
	MAX_RECV_PACKET_CH_SIZE   = 64
	MAX_RECV_STATUS_CH_SIZE   = 64
	MAX_RECV_RESPONSE_CH_SIZE = 64
	MAX_RECV_BYTE_CH_SIZE     = 10000

	DELIMITER        = 0x7E
	TRANSMIT_REQUEST = 0x00
	TRANSMIT_RESULT  = 0x01
	RECEIVED_FRAME   = 0x02
	ADDRESS_REQUEST  = 0x03
	ADDRESS_RESPONSE = 0x04
	SEARCH_REQUEST   = 0x05
	SEARCH_RESPONSE  = 0x06

	FRAME_BCST = 0x00
	FRAME_P2P  = 0x01

	DELIMITER_OFFSET     = 0
	LENGTH_HIGH_OFFSET   = 1
	LENGTH_LOW_OFFSET    = 2
	FRAME_SEQ_OFFSET     = 3
	FRAME_TYPE_OFFSET    = 4
	TRANSMIT_TYPE_OFFSET = 5

	ADDR_PAYLOAD_OFFSET = 5

	RESULT_STATUS_OFFSET = 5
	RESULT_OK            = 0x00
	RESULT_FAIL          = 0x01
)

var child_ctx, cancel = context.WithCancel(context.TODO())

type Reader struct {
	Port         *serial.Port
	RecvRespCh   chan []byte
	RecvPacketCh chan []byte
	RecvByteCh   chan byte
	RecvStatusCh chan []byte
	started      bool

	Mutex sync.Mutex
	wg    sync.WaitGroup
}

func NewReader(port *serial.Port) *Reader {
	return &Reader{
		Port:         port,
		RecvPacketCh: make(chan []byte, MAX_RECV_PACKET_CH_SIZE),
		RecvStatusCh: make(chan []byte, MAX_RECV_STATUS_CH_SIZE),
		RecvRespCh:   make(chan []byte, MAX_RECV_RESPONSE_CH_SIZE),
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
				return nil, errors.New("get status timeout")
			}
		}
	}
}

func (reader *Reader) validate(frame []byte) bool {
	crc := 0xFF
	for i := 3; i < len(frame)-1; i++ {
		crc ^= int(frame[i])
	}
	return crc == int(frame[len(frame)-1])
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
				len_high := int(frame[LENGTH_HIGH_OFFSET])
				len_low := int(frame[LENGTH_LOW_OFFSET])
				length := len_high/16*16*16*16 + len_high%16*16*16 + len_low/16*16 + len_low%16
				frame = append(frame, reader.ReadBytes(ctx, length)...)
				// print(frame)
				if reader.validate(frame) {
					temp := make([]byte, len(frame))
					copy(temp, frame)
					if frame[FRAME_TYPE_OFFSET] == TRANSMIT_RESULT {
						select {
						case reader.RecvStatusCh <- temp:
							// print(temp)
						default:
							<-reader.RecvStatusCh
							reader.RecvStatusCh <- temp
						}
					} else if frame[FRAME_TYPE_OFFSET] == RECEIVED_FRAME {
						select {
						case reader.RecvPacketCh <- temp:
						default:
							<-reader.RecvPacketCh
							reader.RecvPacketCh <- temp
						}
					} else if frame[FRAME_TYPE_OFFSET] == ADDRESS_RESPONSE ||
						frame[FRAME_TYPE_OFFSET] == SEARCH_RESPONSE {
						select {
						case reader.RecvRespCh <- temp:
							// print(temp)
						default:
							<-reader.RecvRespCh
							reader.RecvRespCh <- temp
						}
					}
				} else {
					// print(frame)
					fmt.Println("received a invalid frame")
				}
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
		reader.Port.Read(data)
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
	reader.wg.Add(2)
	child_ctx, cancel = context.WithCancel(ctx)
	go reader.ReadByte(child_ctx)
	go reader.ReadFrame(child_ctx)
}

func (reader *Reader) Stop() {
	reader.Clear()
	cancel()
	reader.wg.Wait()
	reader.started = false
}
