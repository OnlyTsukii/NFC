package nfd

import (
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
)

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

func IPv4ToHex(ip string) []byte {
	parts := strings.Split(ip, ".")
	hexParts := make([]byte, 0)
	for _, part := range parts {
		decimal, err := strconv.ParseInt(part, 10, 16)
		if err != nil {
			logger.Warnf("Invalid decimal string")
			return nil
		}
		hexParts = append(hexParts, byte(decimal))
	}
	return hexParts
}

func HexToIPv4(hex []byte) string {
	var ipParts []string
	for i := range hex {
		ipParts = append(ipParts, strconv.Itoa(int(hex[i])))
	}
	return strings.Join(ipParts, ".")
}

func IPv6ToHex(ip string) []byte {
	parts := strings.Split(ip, ":")
	hexParts := make([]byte, 0)
	for _, part := range parts {
		if len(part) == 0 {
			part = "0000"
		} else if len(part) == 1 {
			part = "0" + part + "00"
		} else if len(part) == 2 {
			part = "00" + part
		} else if len(part) == 3 {
			part = "0" + part[:1] + part[1:]
		}
		hexBytes, err := hex.DecodeString(part[:2])
		if err != nil {
			logger.Warnf("Invalid decimal string")
			return nil
		}
		hexParts = append(hexParts, hexBytes...)
		hexBytes, err = hex.DecodeString(part[2:])
		if err != nil {
			logger.Warnf("Invalid decimal string")
			return nil
		}
		hexParts = append(hexParts, hexBytes...)
	}
	return hexParts
}

func HexToIPv6(hex []byte) string {
	var ipParts []string
	for len(hex) > 0 {
		str1 := strconv.FormatInt(int64(hex[0]), 16)
		str2 := strconv.FormatInt(int64(hex[1]), 16)
		if str1 == "0" {
			if str2 == "0" {
				ipParts = append(ipParts, "")
			} else {
				ipParts = append(ipParts, str2)
			}
		} else {
			if len(str2) == 1 {
				str2 = "0" + str2
			}
			ipParts = append(ipParts, str1+str2)
		}
		hex = hex[2:]
	}
	return strings.Join(ipParts, ":")
}

func GetIP(packet []byte) (string, string, error) {
	version := packet[0] / 16
	if version == 4 {
		return HexToIPv4(packet[12:16]), HexToIPv4(packet[16:20]), nil
	} else if version == 6 {
		return HexToIPv6(packet[8:24]), HexToIPv6(packet[24:40]), nil
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

// func GetIPData(packet []byte) ([]byte, error) {
// 	version := packet[0] / 16
// 	if version == 4 {
// 		return packet[20:], nil
// 	} else if version == 6 {
// 		return packet[40:], nil
// 	} else {
// 		return nil, errors.New("wrong packet version")
// 	}
// }

// func Ping(hostname string, msg []byte) ([]byte, error) {
// 	ipAddr, err := net.ResolveIPAddr("ip4", hostname)
// 	if err != nil {
// 		logger.Warnf("Error resolving IP address:", err)
// 		return nil, err
// 	}

// 	conn, err := net.DialIP("ip4:icmp", nil, ipAddr)
// 	if err != nil {
// 		logger.Warnf("Error creating ICMP connection:", err)
// 		return nil, err
// 	}
// 	defer conn.Close()

// 	start := time.Now()
// 	_, err = conn.Write(msg)
// 	if err != nil {
// 		logger.Warnf("Error sending ICMP message:", err)
// 		return nil, err
// 	}

// 	reply := make([]byte, len(msg))
// 	err = conn.SetReadDeadline(time.Now().Add(time.Second * 3))
// 	if err != nil {
// 		logger.Warnf("Error setting read deadline:", err)
// 		return nil, err
// 	}
// 	_, err = conn.Read(reply)
// 	if err != nil {
// 		logger.Warnf("Error reading ICMP reply:", err)
// 		return nil, err
// 	}

// 	duration := time.Since(start)
// 	logger.Infof("Ping %s (%s): %d bytes, time=%s\n", hostname, ipAddr, len(reply), duration)
// 	return reply, nil
// }

// func GetMsg() []byte {
// 	msg := make([]byte, 48)
// 	msg[0] = 8
// 	msg[1] = 0
// 	msg[2] = 0
// 	msg[3] = 0
// 	msg[4] = 0
// 	msg[5] = 13
// 	msg[6] = 0
// 	msg[7] = 37

// 	checksum := checkSum(msg)
// 	msg[2] = byte(checksum >> 8)
// 	msg[3] = byte(checksum)

// 	return msg
// }

// func checkSum(msg []byte) uint16 {
// 	sum := 0
// 	for i := 0; i < len(msg)-1; i += 2 {
// 		sum += int(msg[i])*256 + int(msg[i+1])
// 	}
// 	sum = (sum >> 16) + (sum & 0xffff)
// 	sum += sum >> 16
// 	return uint16(^sum)
// }
