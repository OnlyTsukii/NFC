package main

import (
	"context"
	"net"
	"time"

	"gitee.com/ccl0924/nfd/nfd"

	slog "gitee.com/czy_hit/log"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

const (
	BatchSize = 100
	BufSize   = 1024
)

var configCh = make(chan nfd.ConfigInfo)
var deviceCh = make(chan nfd.DeviceInfo)
var strategy = nfd.Strategy{BatchSize: BatchSize}
var log slog.Logger
var err error
var selfIP = []byte{192, 168, 101, 1}

// var AddrMap = map[string]string{
// 	"0013a20041bb76a4": "192.168.148.130:8001",
// 	"0013a20041bb7684": "192.168.148.133:8001",
// }

// var ListenPort = 8001

// func StartDeviceA(ctx context.Context) *nfd.NearFieldDevice {
// 	n := nfd.NewNearFieldDevice("192.168.101.1", "", "0013a20041bb76a4", AddrMap, ListenPort)
// 	n.Init(strategy)
// 	n.Run(ctx, configCh, deviceCh)
// 	return n
// }

func StartDeviceA(ctx context.Context) *nfd.NearFieldDevice {
	n := nfd.NewNearFieldDevice("192.168.101.1", "")
	n.Init(strategy)
	n.Run(ctx, configCh, deviceCh)
	return n
}

func init() {
	log, err = slog.NewLogger()
	if err != nil {
		panic(err)
	}
}

func listenAndPrint(ctx context.Context, d *nfd.NearFieldDevice) {
	buf := make([]byte, BufSize)
	size := make([]int, BatchSize)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		d.Read(buf)
		if size[0] > 0 {
			log.Info("read a packet from buffer: len ", size[0])
			// Decode a packet
			packet := gopacket.NewPacket(buf[:size[0]], layers.LayerTypeIPv4, gopacket.Default)
			// Get the IPv4 layer from this packet
			if ipv4Layer := packet.Layer(layers.LayerTypeIPv4); ipv4Layer != nil {
				// Get actual IPv4 data from this layer
				ipv4, _ := ipv4Layer.(*layers.IPv4)
				dstIP := ipv4.DstIP
				if !dstIP.Equal(selfIP) {
					reply, err := Ping(dstIP.String(), packet.Data()[20:])
					if err != nil {
						log.Warn(err)
					} else {
						err := SendIP(d, dstIP, ipv4.SrcIP, reply[20:])
						if err != nil {
							log.Warn(err)
						}
					}
				}
			}
		}
		size[0] = 0
	}
}

func DeviceInfoHandler(ctx context.Context, d *nfd.NearFieldDevice) {
	for {
		select {
		case info := <-deviceCh:
			if info.Type == nfd.STATUS_QUERY_REQ {
				configCh <- nfd.ConfigInfo{Type: nfd.STATUS_QUERY_RESP, Status: 1, SrcMac: info.SrcMac}
			} else if info.Type == nfd.SERACH_NODES_RESP {
				flag := false
				for key := range info.NodesStatus {
					if info.NodesStatus[key] == 1 {
						flag = true
						break
					}
				}
				if flag {
					err := SendIP(d, selfIP, []byte{180, 101, 50, 242}, GetMsg())
					if err != nil {
						log.Error(err)
					}
				}
			}
		default:
		}
	}
}

func main() {

	//defer log.Sync()
	//
	//ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	//defer stop()
	//
	//deviceA := StartDeviceA(ctx)
	//
	//go listenAndPrint(ctx, deviceA)
	//
	//go DeviceInfoHandler(ctx, deviceA)
	//
	//time.Sleep(10 * time.Second)

	//configCh <- nfd.ConfigInfo{Type: nfd.SERACH_NODES_REQ, Status: 0, SrcMac: ""}

	//time.Sleep(30 * time.Second)

	//stop()
	//
	//<-ctx.Done()

	// deviceA := StartDeviceA(ctx)
	// // messageA := []byte("message from A")
	// messageA := data

	// buf := gopacket.NewSerializeBuffer()
	// opts := gopacket.SerializeOptions{}

	// go listenAndPrint(ctx, deviceA)

	// time.Sleep(5 * time.Second)
	// log.Info("start send test")

	// bufs := make([][]byte, BatchSize)

	// gopacket.SerializeLayers(buf, opts,
	// 	&layers.IPv4{SrcIP: net.IPv4(192, 168, 101, 1), DstIP: net.IPv4(192, 168, 101, 2)},
	// 	&layers.UDP{SrcPort: 2333, DstPort: 2333},
	// 	gopacket.Payload(messageA))
	// for i := 0; i < BatchSize; i++ {
	// 	bufs[i] = buf.Bytes()
	// 	bufs[i][0] = (4 << 4)
	// }
	// // bufs[0] = buf.Bytes()
	// // bufs[0][0] = (4 << 4)
	// // log.Info(bufs[0])
	// n, err := deviceA.Write(bufs, 0)
	// if err != nil {
	// 	log.Error(err)
	// }
	// log.Infof("send %d message success\n", n)

	// time.Sleep(5 * time.Second)

	// stop()

	// <-ctx.Done()
}

func SendIP(n *nfd.NearFieldDevice, srcIP []byte, dstIP []byte, msg []byte) error {
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
	_, err := n.Write(data)
	if err != nil {
		return err
	}
	return nil
}

func Ping(hostname string, msg []byte) ([]byte, error) {
	ipAddr, err := net.ResolveIPAddr("ip4", hostname)
	if err != nil {
		log.Warnf("Error resolving IP address:", err)
		return nil, err
	}

	conn, err := net.DialIP("ip4:icmp", nil, ipAddr)
	if err != nil {
		log.Warnf("Error creating ICMP connection:", err)
		return nil, err
	}
	defer conn.Close()

	start := time.Now()
	_, err = conn.Write(msg)
	if err != nil {
		log.Warnf("Error sending ICMP message:", err)
		return nil, err
	}

	reply := make([]byte, len(msg))
	err = conn.SetReadDeadline(time.Now().Add(time.Second * 3))
	if err != nil {
		log.Warnf("Error setting read deadline:", err)
		return nil, err
	}
	num, err := conn.Read(reply)
	log.Info(num)
	if err != nil {
		log.Warnf("Error reading ICMP reply:", err)
		return nil, err
	}

	duration := time.Since(start)
	log.Infof("Ping %s (%s): %d bytes, time=%s\n", hostname, ipAddr, len(reply), duration)
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
