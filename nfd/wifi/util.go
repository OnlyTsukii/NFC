package wifi

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

func calcCheckSum(frame []byte) byte {
	crc := 0xFF
	for i := 3; i < len(frame); i++ {
		crc ^= int(frame[i])
	}
	return byte(crc)
}

func validate(frame []byte) bool {
	crc := 0xFF
	for i := 3; i < len(frame)-1; i++ {
		crc ^= int(frame[i])
	}
	return crc == int(frame[len(frame)-1])
}

func StrToBytes(strAddr string) []byte {
	bytes, _ := hex.DecodeString(strAddr)
	return bytes
}

func BytesToStr(bs []byte) string {
	hexString := hex.EncodeToString(bs)
	return hexString
}
