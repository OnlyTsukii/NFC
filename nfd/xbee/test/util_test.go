package xbee_test

import (
	"ccl/go/nfd/xbee"
	"encoding/hex"
	"reflect"
	"testing"
)

func TestATCmdFrameEncodingDecoding(t *testing.T) {
	testFrame := &xbee.ATCmdFrame{
		Delimiter: 0x7E,
		LenStart:  0x00,
		LenEnd:    0x08,
		FrameType: 0x08,
		FrameSeq:  0x01,
		Data:      []byte{0x41, 0x54, 0x43, 0x01},
		CheckSum:  0xBE,
	}

	encodedFrame := xbee.ATCmdFrameToBytes(testFrame)

	decodedFrame := &xbee.ATCmdFrame{}
	decodedFrame.Delimiter = encodedFrame[0]
	decodedFrame.LenStart = encodedFrame[1]
	decodedFrame.LenEnd = encodedFrame[2]
	decodedFrame.FrameType = encodedFrame[3]
	decodedFrame.FrameSeq = encodedFrame[4]
	decodedFrame.Data = encodedFrame[5 : len(encodedFrame)-1]
	decodedFrame.CheckSum = encodedFrame[len(encodedFrame)-1]

	if !reflect.DeepEqual(testFrame, decodedFrame) {
		t.Errorf("Decoded frame is not equal to the original frame")
	}
}

func TestTransmitRequestFrameEncodingDecoding(t *testing.T) {
	testFrame := &xbee.TransmitRequestFrame{
		Delimiter: 0x7E,
		LenStart:  0x00,
		LenEnd:    0x11,
		FrameType: 0x10,
		FrameSeq:  0x01,
		X64Addr:   []byte{0x00, 0x13, 0xA2, 0x00, 0x40, 0x12, 0x34, 0x56},
		X16Addr:   []byte{0xFF, 0xFE},
		BR:        0x00,
		Options:   0x00,
		Data:      []byte{0x41, 0x54, 0x43, 0x01},
		CheckSum:  0xBE,
	}

	encodedFrame := xbee.TransmitRequestFrameToBytes(testFrame)

	decodedFrame := &xbee.TransmitRequestFrame{}
	decodedFrame.Delimiter = encodedFrame[0]
	decodedFrame.LenStart = encodedFrame[1]
	decodedFrame.LenEnd = encodedFrame[2]
	decodedFrame.FrameType = encodedFrame[3]
	decodedFrame.FrameSeq = encodedFrame[4]
	decodedFrame.X64Addr = encodedFrame[5:13]
	decodedFrame.X16Addr = encodedFrame[13:15]
	decodedFrame.BR = encodedFrame[15]
	decodedFrame.Options = encodedFrame[16]
	decodedFrame.Data = encodedFrame[17 : len(encodedFrame)-1]
	decodedFrame.CheckSum = encodedFrame[len(encodedFrame)-1]

	if !reflect.DeepEqual(testFrame, decodedFrame) {
		t.Errorf("Decoded frame is not equal to the original frame")
	}
}

func TestCheckSumGeneration(t *testing.T) {
	data := []byte{0x08, 0x01, 0x41, 0x54, 0x43, 0x01}
	checkSum := xbee.GenCheckSum(data)
	if checkSum != 0x68 {
		t.Errorf("Generated checksum does not match the expected value")
	}
}

func TestStrToBytesAndBytesToStr(t *testing.T) {
	strAddr := "0013a20040123456"
	expectedBytes, _ := hex.DecodeString(strAddr)

	convertedBytes := xbee.StrToBytes(strAddr)
	if !reflect.DeepEqual(expectedBytes, convertedBytes) {
		t.Errorf("Converted bytes do not match the expected bytes")
	}

	convertedStr := xbee.BytesToStr(expectedBytes)
	if convertedStr != strAddr {
		t.Errorf("Converted string does not match the original string")
	}
}
