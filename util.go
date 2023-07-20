package main

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

func print(bs []byte) {
	for _, v := range bs {
		fmt.Printf("%02x", v)
	}
	fmt.Println()
}

func ip2hex(ip string) string {
	parts := strings.Split(ip, ".")
	hexParts := make([]string, len(parts))
	for i, part := range parts {
		decimal, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			fmt.Println("Invalid decimal string")
			return ""
		}
		hexStr := strconv.FormatInt(decimal, 16)
		if len(hexStr) == 1 {
			hexStr = "0" + hexStr
		}
		hexParts[i] = hexStr
	}
	return strings.Join(hexParts, "")
}

func hex2ip(hex string) string {
	// fmt.Println(hex)
	var ipParts []string
	for len(hex) > 0 {
		decimal, err := strconv.ParseInt(hex[:2], 16, 64)
		if err != nil {
			fmt.Println(err)
			return ""
		}
		decimalStr := strconv.FormatInt(decimal, 10)
		ipParts = append(ipParts, decimalStr)
		hex = hex[2:]
	}
	return strings.Join(ipParts, ".")
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
	if resp[FRAME_SEQ_OFFSET] != AT.FrameSeq {
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
