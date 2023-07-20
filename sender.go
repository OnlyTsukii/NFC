package main

import (
	"errors"
	"time"

	"github.com/tarm/serial"
)

const (
	MAX_FRAME_DATA_LEN = 241
	BCST_X16_ADDR      = "FFFE"
)

type Sender struct {
	Port serial.Port
	Seq  int
}

func NewSender(port serial.Port) *Sender {
	return &Sender{
		Port: port,
		Seq:  0,
	}
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

func (s *Sender) GenATCmd(cmd string) *ATCmdFrame {
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

func (s *Sender) GenTransmitRequest(x64addr []byte, data []byte) *TransmitRequestFrame {
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
		Options:   0x00,
		Data:      data,
	}
	CMD.CheckSum = GenCheckSum(TransmitRequestFrameToBytes(&CMD))
	return &CMD
}

func (s *Sender) SendATCmdWithResponse(cmd []string, r *Reader) (map[string][]byte, error) {
	resp := make(map[string][]byte, 0)
	ATs := make([]ATCmdFrame, len(cmd))
	for i, v := range cmd {
		AT := s.GenATCmd(v)
		ATs[i] = *AT
		s.Port.Write(ATCmdFrameToBytes(AT))
		time.Sleep(50 * time.Millisecond)
	}
	for _, v := range ATs {
		res, err := r.GetResp(RESPONSE_TIMEOUT)
		if err != nil {
			return nil, err
		}
		if isATCmdValid(res, &v) {
			end := 3 + int(res[LEN_END_OFFSET])
			resp[string(v.Data)] = res[AT_RESP_RESPONSE_OFFSET:end]
		} else {
			return nil, errors.New("received response is invalid")
		}
	}
	return resp, nil
}

func (s *Sender) SendPacketWithResponse(x64addr string, data []byte, r *Reader) error {
	if len(data) > MAX_FRAME_DATA_LEN {
		return errors.New("data too long")
	}
	CMD := s.GenTransmitRequest(StrToBytes(x64addr), data)
	packet := TransmitRequestFrameToBytes(CMD)
	s.Port.Write(packet)
	// time.Sleep(200 * time.Millisecond)
	resp, err := r.GetResp(RESPONSE_TIMEOUT)
	if err != nil {
		return err
	}
	if isTransmitStatusValid(resp, CMD) {
		return nil
	} else {
		return errors.New("received response is invalid")
	}
}
