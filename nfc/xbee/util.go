package xbee

import (
	"encoding/hex"
	"fmt"
)

func print(bs []byte) {
	for _, v := range bs {
		fmt.Printf("%02x", v)
	}
	fmt.Println()
}

func ATCmdFrameToBytes(f *ATCmdFrame) []byte {
	bs := []byte{
		f.Delimiter,
		f.LenStart,
		f.LenEnd,
		f.FrameType,
		f.FrameSeq,
	}
	bs = append(bs, f.Data...)
	bs = append(bs, f.CheckSum)
	return bs
}

func TransmitRequestFrameToBytes(f *TransmitRequestFrame) []byte {
	bs := []byte{
		f.Delimiter,
		f.LenStart,
		f.LenEnd,
		f.FrameType,
		f.FrameSeq,
	}
	bs = append(bs, f.X64Addr...)
	bs = append(bs, f.X16Addr...)
	bs = append(bs, f.BR)
	bs = append(bs, f.Options)
	bs = append(bs, f.Data...)
	bs = append(bs, f.CheckSum)
	return bs
}

func GenCheckSum(bs []byte) byte {
	var checksum byte
	for _, b := range bs[3 : len(bs)-1] {
		checksum += b
	}
	checksum = 0xFF - (checksum & 0xFF)
	return checksum
}

func Check(bs []byte) bool {
	return bs[len(bs)-1] == GenCheckSum(bs)
}

func StrToBytes(strAddr string) []byte {
	bytes, _ := hex.DecodeString(strAddr)
	return bytes
}

func BytesToStr(bs []byte) string {
	hexString := hex.EncodeToString(bs)
	return hexString
}

func isATCmdValid(resp []byte, AT *ATCmdFrame) bool {
	if resp == nil {
		return false
	}
	if Check(resp) {
		if resp[AT_RESP_STATUS_OFFSET] == 0 {
			return true
		} else {
			return false
		}
	} else {
		return false
	}
}

func isTransmitStatusValid(resp []byte, CMD *TransmitRequestFrame) bool {
	if resp == nil {
		return false
	}
	if resp[FRAME_SEQ_OFFSET] != CMD.FrameSeq {
		return false
	}
	if Check(resp) {
		if resp[TRANSMIT_STATUS_OFFSET] == 0 {
			return true
		} else {
			return false
		}
	} else {
		return false
	}
}
