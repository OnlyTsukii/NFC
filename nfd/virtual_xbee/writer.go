package vxbee

import (
	"errors"
)

const (
	MAX_FRAME_DATA_LEN = 241
	BCST_X16_ADDR      = "FFFE"
)

type Writer struct {
	VirtualDevice *VirtualXbeeDevice
	Seq           int
}

func NewWriter(VirtualDevice *VirtualXbeeDevice) *Writer {
	return &Writer{
		VirtualDevice: VirtualDevice,
		Seq:           0,
	}
}

type ATCmdRespFrame struct {
	Delimiter byte
	LenStart  byte
	LenEnd    byte
	FrameType byte
	FrameSeq  byte
	Command   []byte
	Status    byte
	Data      []byte
	CheckSum  byte
}

type TransmitStatusFrame struct {
	Delimiter       byte
	LenStart        byte
	LenEnd          byte
	FrameType       byte
	FrameSeq        byte
	X16Addr         []byte
	RetryCount      byte
	DeliveryStatus  byte
	DiscoveryStatus byte
	CheckSum        byte
}

type ReceivePacketFrame struct {
	Delimiter byte
	LenStart  byte
	LenEnd    byte
	FrameType byte
	X64Addr   []byte
	X16Addr   []byte
	Options   byte
	Data      []byte
	CheckSum  byte
}

type ATCmdFrame struct {
	Delimiter byte
	LenStart  byte
	LenEnd    byte
	FrameType byte
	FrameSeq  byte
	Data      []byte
	CheckSum  byte
}

type TransmitRequestFrame struct {
	Delimiter byte
	LenStart  byte
	LenEnd    byte
	FrameType byte
	FrameSeq  byte
	X64Addr   []byte
	X16Addr   []byte
	BR        byte
	Options   byte
	Data      []byte
	CheckSum  byte
}

func GenATCmdResp(cmd []byte, seq byte, data []byte) *ATCmdRespFrame {
	end := 1 + 1 + len(cmd) + 1 + len(data)
	CMD := ATCmdRespFrame{
		Delimiter: 0x7e,
		LenStart:  0x00,
		LenEnd:    byte(end),
		FrameType: 0x88,
		FrameSeq:  seq,
		Command:   cmd,
		Status:    0x00,
		Data:      data,
	}
	CMD.CheckSum = GenCheckSum(ATCmdRespFrameToBytes(&CMD))
	return &CMD
}

func GenTransmitStatus(seq byte) *TransmitStatusFrame {
	CMD := TransmitStatusFrame{
		Delimiter:       0x7e,
		LenStart:        0x00,
		LenEnd:          0x07,
		FrameType:       0x8B,
		FrameSeq:        seq,
		X16Addr:         []byte{0xFF, 0xFE},
		RetryCount:      0x01,
		DeliveryStatus:  0x00,
		DiscoveryStatus: 0x00,
	}
	CMD.CheckSum = GenCheckSum(TransmitStatusFrameToBytes(&CMD))
	return &CMD
}

func GenReceivePacket(x64addr []byte, data []byte) *ReceivePacketFrame {
	end := 1 + 8 + 2 + 1 + len(data)
	CMD := ReceivePacketFrame{
		Delimiter: 0x7e,
		LenStart:  0x00,
		LenEnd:    byte(end),
		FrameType: 0x90,
		X64Addr:   x64addr,
		X16Addr:   []byte{0xFF, 0xFE},
		Options:   0x01,
		Data:      data,
	}
	CMD.CheckSum = GenCheckSum(ReceivePacketFrameToBytes(&CMD))
	return &CMD
}

func (s *Writer) GenATCmd(cmd string) *ATCmdFrame {
	CMD := ATCmdFrame{
		Delimiter: 0x7e,
		LenStart:  0x00,
		LenEnd:    0x04,
		FrameType: 0x09,
	}
	s.Seq = (s.Seq + 1) % 256
	CMD.FrameSeq = byte(s.Seq)
	for _, v := range cmd {
		CMD.Data = append(CMD.Data, byte(v))
	}
	CMD.CheckSum = GenCheckSum(ATCmdFrameToBytes(&CMD))
	return &CMD
}

func (s *Writer) GenTransmitRequest(x64addr []byte, data []byte) *TransmitRequestFrame {
	end := 1 + 1 + 8 + 2 + 1 + 1 + len(data)
	s.Seq = (s.Seq + 1) % 256
	CMD := TransmitRequestFrame{
		Delimiter: 0x7e,
		LenStart:  0x00,
		LenEnd:    byte(end),
		FrameType: 0x10,
		FrameSeq:  byte(s.Seq),
		X64Addr:   x64addr,
		X16Addr:   StrToBytes(BCST_X16_ADDR),
		BR:        0x00,
		Options:   0x08,
		Data:      data,
	}
	CMD.CheckSum = GenCheckSum(TransmitRequestFrameToBytes(&CMD))
	return &CMD
}

func (s *Writer) GetNodes(r *Reader) ([][]byte, error) {
	resp := make([][]byte, 0)
	AT := s.GenATCmd("ND")
	s.VirtualDevice.Write(ATCmdFrameToBytes(AT), "")
	timeout := 13
	for {
		frame, err := r.GetResp(timeout, AT.FrameSeq)
		if err != nil {
			return resp, nil
		} else {
			timeout = RESPONSE_TIMEOUT
			if isATCmdValid(frame, AT) {
				resp = append(resp, frame[AT_RESP_RESPONSE_OFFSET+2:AT_RESP_RESPONSE_OFFSET+10])
			} else {
				return nil, errors.New("received response is invalid")
			}
		}
	}
}

func (s *Writer) SendATCmdWithResponse(cmd []string, r *Reader) (map[string][]byte, error) {
	resp := make(map[string][]byte, 0)
	for i, v := range cmd {
		AT := s.GenATCmd(v)
		s.VirtualDevice.Write(ATCmdFrameToBytes(AT), "")
		res, err := r.GetResp(RESPONSE_TIMEOUT, AT.FrameSeq)
		if err != nil {
			return nil, err
		}
		if isATCmdValid(res, AT) {
			end := 3 + int(res[LEN_END_OFFSET])
			resp[cmd[i]] = res[AT_RESP_RESPONSE_OFFSET:end]
		} else {
			return nil, errors.New("received response is invalid")
		}
	}
	return resp, nil
}

func (s *Writer) SendPacketWithResponse(x64addr string, data []byte, r *Reader) error {
	if len(data) > MAX_FRAME_DATA_LEN {
		return errors.New("data too long")
	}
	CMD := s.GenTransmitRequest(StrToBytes(x64addr), data)
	packet := TransmitRequestFrameToBytes(CMD)
	err := s.VirtualDevice.Write(packet, x64addr)
	if err != nil {
		return err
	}
	resp, err := r.GetStatus(RESPONSE_TIMEOUT, CMD.FrameSeq)
	if err != nil {
		return err
	}
	if isTransmitStatusValid(resp, CMD) {
		return nil
	} else {
		return errors.New("received response is invalid")
	}
}
