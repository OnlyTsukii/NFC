package nfd

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"net"
	"strconv"
	"strings"
	"time"
)

func myPrint(bs []byte) {
	for _, v := range bs {
		fmt.Printf("%02x", v)
	}
	fmt.Println()
}

func MacToHex(mac string) ([]byte, error) {
	hexParts := make([]byte, 0)
	for len(mac) > 0 {
		hexBytes, err := hex.DecodeString(mac[:2])
		if err != nil {
			logger.Warnf("Invalid decimal string")
			return nil, err
		}
		hexParts = append(hexParts, hexBytes...)
		mac = mac[2:]
	}
	return hexParts, nil
}

func HexToMac(hex []byte) string {
	var ipParts []string
	for i := range hex {
		str := strconv.FormatInt(int64(hex[i]), 16)
		if len(str) == 1 {
			str = "0" + str
		}
		ipParts = append(ipParts, str)
	}
	return strings.Join(ipParts, "")
}

func IPv4ToHex(ipv4Str string) []byte {
	ip := net.ParseIP(ipv4Str)
	if ip == nil {
		return nil
	}
	return ip.To4()
}

func HexToIPv4(ipv4Bytes []byte) string {
	if len(ipv4Bytes) != net.IPv4len {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d.%d", ipv4Bytes[0], ipv4Bytes[1], ipv4Bytes[2], ipv4Bytes[3])
}

func IPv6ToHex(ipv6Str string) []byte {
	ip := net.ParseIP(ipv6Str)
	if ip == nil {
		return nil
	}
	return ip.To16()
}

func HexToIPv6(ipv6Bytes []byte) string {
	ip := net.IP(ipv6Bytes)
	return ip.String()
}

func GetIP(packet []byte) (string, string, error) {
	version := packet[0] / 16
	if version == 4 {
		return HexToIPv4(packet[12:16]), HexToIPv4(packet[16:20]), nil
	} else if version == 6 {
		//return HexToIPv6(packet[8:24]), HexToIPv6(packet[24:40]), nil
		return "", "", errors.New("ipv6 is not supported")
	} else {
		return "", "", errors.New("wrong packet version")
	}
}

func CreateIPData(n *NearFieldDevice, dest string, bs []byte) []byte {
	if len(dest) > 15 {
		data := make([]byte, 40+len(bs))
		data[0] = byte(0x60)
		copy(data[1:8], []byte{0x00, 0x00, 0x00, 0x00, 0x20, 0x3a, 0xff})
		copy(data[8:24], IPv6ToHex(n.IPv6))
		copy(data[24:40], IPv6ToHex(dest))
		if len(bs) > 0 {
			copy(data[40:], bs)
		}
		return data
	} else {
		data := make([]byte, 20+len(bs))
		data[0] = byte(0x45)
		copy(data[1:12], []byte{0x00, 0x00, 0x14, 0x2e, 0x6d, 0x00, 0x00, 0x01, 0x11, 0x00, 0x00})
		copy(data[12:16], IPv4ToHex(n.IPv4))
		copy(data[16:20], IPv4ToHex(dest))
		if len(bs) > 0 {
			copy(data[20:], bs)
		}
		return data
	}
}

func GetIPData(srcIP []byte, dstIP []byte, msg []byte) []byte {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{}
	gopacket.SerializeLayers(buf, opts,
		&layers.IPv4{
			SrcIP: net.IPv4(srcIP[0], srcIP[1], srcIP[2], srcIP[3]),
			DstIP: net.IPv4(dstIP[0], dstIP[1], dstIP[2], dstIP[3]),
		},
		gopacket.Payload(msg))
	data := buf.Bytes()
	data[0] = (4 << 4)
	return data
}

func Ping(hostname string, msg []byte) ([]byte, error) {
	ipAddr, err := net.ResolveIPAddr("ip4", hostname)
	if err != nil {
		logger.Warnf("Error resolving IP address:", err)
		return nil, err
	}

	conn, err := net.DialIP("ip4:icmp", nil, ipAddr)
	if err != nil {
		logger.Warnf("Error creating ICMP connection:", err)
		return nil, err
	}
	defer conn.Close()

	start := time.Now()
	_, err = conn.Write(msg)
	if err != nil {
		logger.Warnf("Error sending ICMP message:", err)
		return nil, err
	}

	reply := make([]byte, len(msg))
	err = conn.SetReadDeadline(time.Now().Add(time.Second * 3))
	if err != nil {
		logger.Warnf("Error setting read deadline:", err)
		return nil, err
	}
	num, err := conn.Read(reply)
	logger.Info(num)
	if err != nil {
		logger.Warnf("Error reading ICMP reply:", err)
		return nil, err
	}

	duration := time.Since(start)
	logger.Infof("Ping %s (%s): %d bytes, time=%s\n", hostname, ipAddr, len(reply), duration)
	return reply, nil
}

func GetMsg() []byte {
	msg := make([]byte, 48)
	msg[0] = 8
	msg[1] = 0
	msg[2] = 0
	msg[3] = 0
	msg[4] = 0
	msg[5] = 13
	msg[6] = 0
	msg[7] = 37

	checksum := checkSum(msg)
	msg[2] = byte(checksum >> 8)
	msg[3] = byte(checksum)

	return msg
}

func checkSum(msg []byte) uint16 {
	sum := 0
	for i := 0; i < len(msg)-1; i += 2 {
		sum += int(msg[i])*256 + int(msg[i+1])
	}
	sum = (sum >> 16) + (sum & 0xffff)
	sum += sum >> 16
	return uint16(^sum)
}
