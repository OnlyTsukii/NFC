package nfd_test

import (
	"bytes"
	"encoding/hex"
	"gitee.com/ccl0924/nfd/nfd"
	"testing"
)

func TestMacToHexAndHexToMac(t *testing.T) {
	mac := "0011223344556677"
	expectedHex := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77}

	hexData := nfd.MacToHex(mac)
	if !bytes.Equal(hexData, expectedHex) {
		t.Errorf("MacToHex returned incorrect result")
	}

	backToMac := nfd.HexToMac(hexData)
	if backToMac != mac {
		t.Errorf("HexToMac returned incorrect result")
	}
}

func TestIPv4ToHexAndHexToIPv4(t *testing.T) {
	ip := "192.168.1.1"
	expectedHex := []byte{0xc0, 0xa8, 0x01, 0x01}

	hexData := nfd.IPv4ToHex(ip)
	if !bytes.Equal(hexData, expectedHex) {
		t.Errorf("IPv4ToHex returned incorrect result")
	}

	backToIP := nfd.HexToIPv4(hexData)
	if backToIP != ip {
		t.Errorf("HexToIPv4 returned incorrect result")
	}
}

func TestIPv6ToHexAndHexToIPv6(t *testing.T) {
	ip := "2001:db8:abcd:ef01:2345:6789:abcd:ef01"
	expectedHex, _ := hex.DecodeString("20010db8abcdef0123456789abcdef01")

	hexData := nfd.IPv6ToHex(ip)
	if !bytes.Equal(hexData, expectedHex) {
		t.Errorf("IPv6ToHex returned incorrect result")
	}

	backToIP := nfd.HexToIPv6(hexData)
	if backToIP != ip {
		t.Errorf("HexToIPv6 returned incorrect result")
	}
}
