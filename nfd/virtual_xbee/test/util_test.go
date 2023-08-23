package vxbee_test

import (
	"encoding/hex"
	vxbee "gitee.com/ccl0924/nfd/nfd/virtual_xbee"
	"gitee.com/ccl0924/nfd/nfd/xbee"
	"reflect"
	"testing"
)

func TestTransmitRequestFrameEncodingDecoding(t *testing.T) {
	testFrame := &vxbee.TransmitRequestFrame{
		Delimiter: 0x7E,
		LenStart:  0x00,
		LenEnd:    0x0D,
		FrameType: 0x10,
		FrameSeq:  0x03,
		X64Addr:   []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x23, 0x45},
		X16Addr:   []byte{0xFF, 0xFE},
		BR:        0x00,
		Options:   0x00,
		Data:      []byte{0x48, 0x65, 0x6C, 0x6C, 0x6F},
		CheckSum:  0x83,
	}

	encodedFrame := vxbee.TransmitRequestFrameToBytes(testFrame)

	decodedFrame := &vxbee.TransmitRequestFrame{
		Delimiter: encodedFrame[0],
		LenStart:  encodedFrame[1],
		LenEnd:    encodedFrame[2],
		FrameType: encodedFrame[3],
		FrameSeq:  encodedFrame[4],
		X64Addr:   encodedFrame[5:13],
		X16Addr:   encodedFrame[13:15],
		BR:        encodedFrame[15],
		Options:   encodedFrame[16],
		Data:      encodedFrame[17 : len(encodedFrame)-1],
		CheckSum:  encodedFrame[len(encodedFrame)-1],
	}

	if !reflect.DeepEqual(testFrame, decodedFrame) {
		t.Errorf("Decoded TransmitRequest frame is not equal to the original frame")
	}
}

func TestATCmdRespFrameEncodingDecoding(t *testing.T) {
	testFrame := &vxbee.ATCmdRespFrame{
		Delimiter: 0x7E,
		LenStart:  0x00,
		LenEnd:    0x0B,
		FrameType: 0x88,
		FrameSeq:  0x02,
		Command:   []byte{0x41, 0x54},
		Status:    0x00,
		Data:      []byte{0x4C, 0x4F, 0x4B},
		CheckSum:  0x85,
	}

	encodedFrame := vxbee.ATCmdRespFrameToBytes(testFrame)

	decodedFrame := &vxbee.ATCmdRespFrame{
		Delimiter: encodedFrame[0],
		LenStart:  encodedFrame[1],
		LenEnd:    encodedFrame[2],
		FrameType: encodedFrame[3],
		FrameSeq:  encodedFrame[4],
		Command:   encodedFrame[5:7],
		Status:    encodedFrame[7],
		Data:      encodedFrame[8 : len(encodedFrame)-1],
		CheckSum:  encodedFrame[len(encodedFrame)-1],
	}

	if !reflect.DeepEqual(testFrame, decodedFrame) {
		t.Errorf("Decoded ATCmdResp frame is not equal to the original frame")
	}
}

func TestATCmdFrameEncodingDecoding(t *testing.T) {
	testFrame := &vxbee.ATCmdFrame{
		Delimiter: 0x7E,
		LenStart:  0x00,
		LenEnd:    0x05,
		FrameType: 0x08,
		FrameSeq:  0x01,
		Data:      []byte{0x41, 0x54, 0x43, 0x01},
		CheckSum:  0xBE,
	}

	encodedFrame := vxbee.ATCmdFrameToBytes(testFrame)

	decodedFrame := &vxbee.ATCmdFrame{
		Delimiter: encodedFrame[0],
		LenStart:  encodedFrame[1],
		LenEnd:    encodedFrame[2],
		FrameType: encodedFrame[3],
		FrameSeq:  encodedFrame[4],
		Data:      encodedFrame[5 : len(encodedFrame)-1],
		CheckSum:  encodedFrame[len(encodedFrame)-1],
	}

	if !reflect.DeepEqual(testFrame, decodedFrame) {
		t.Errorf("Decoded ATCmd frame is not equal to the original frame")
	}
}

func TestReceivePacketFrameEncodingDecoding(t *testing.T) {
	testFrame := &vxbee.ReceivePacketFrame{
		Delimiter: 0x7E,
		LenStart:  0x00,
		LenEnd:    0x0E,
		FrameType: 0x90,
		X64Addr:   []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x23, 0x45},
		X16Addr:   []byte{0x00, 0x01},
		Options:   0x01,
		Data:      []byte{0x48, 0x65, 0x6C, 0x6C, 0x6F},
		CheckSum:  0x84,
	}

	encodedFrame := vxbee.ReceivePacketFrameToBytes(testFrame)

	decodedFrame := &vxbee.ReceivePacketFrame{
		Delimiter: encodedFrame[0],
		LenStart:  encodedFrame[1],
		LenEnd:    encodedFrame[2],
		FrameType: encodedFrame[3],
		X64Addr:   encodedFrame[4:12],
		X16Addr:   encodedFrame[12:14],
		Options:   encodedFrame[14],
		Data:      encodedFrame[15 : len(encodedFrame)-1],
		CheckSum:  encodedFrame[len(encodedFrame)-1],
	}

	if !reflect.DeepEqual(testFrame, decodedFrame) {
		t.Errorf("Decoded ReceivePacket frame is not equal to the original frame")
	}
}

func TestTransmitStatusFrameEncodingDecoding(t *testing.T) {
	testFrame := &vxbee.TransmitStatusFrame{
		Delimiter:       0x7E,
		LenStart:        0x00,
		LenEnd:          0x0C,
		FrameType:       0x89,
		FrameSeq:        0x02,
		X16Addr:         []byte{0x00, 0x01},
		RetryCount:      0x03,
		DeliveryStatus:  0x00,
		DiscoveryStatus: 0x00,
		CheckSum:        0xB9,
	}

	encodedFrame := vxbee.TransmitStatusFrameToBytes(testFrame)

	decodedFrame := &vxbee.TransmitStatusFrame{
		Delimiter:       encodedFrame[0],
		LenStart:        encodedFrame[1],
		LenEnd:          encodedFrame[2],
		FrameType:       encodedFrame[3],
		FrameSeq:        encodedFrame[4],
		X16Addr:         encodedFrame[5:7],
		RetryCount:      encodedFrame[7],
		DeliveryStatus:  encodedFrame[8],
		DiscoveryStatus: encodedFrame[9],
		CheckSum:        encodedFrame[10],
	}

	if !reflect.DeepEqual(testFrame, decodedFrame) {
		t.Errorf("Decoded TransmitStatus frame is not equal to the original frame")
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
