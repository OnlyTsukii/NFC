package main

import (
	"context"
	"net"
	"os/signal"
	"syscall"
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

func StartDevice(ctx context.Context) *nfd.NearFieldDevice {
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
			//// Decode a packet
			//packet := gopacket.NewPacket(buf[:size[0]], layers.LayerTypeIPv4, gopacket.Default)
			//// Get the IPv4 layer from this packet
			//if ipv4Layer := packet.Layer(layers.LayerTypeIPv4); ipv4Layer != nil {
			//	// Get actual IPv4 data from this layer
			//	ipv4, _ := ipv4Layer.(*layers.IPv4)
			//	dstIP := ipv4.DstIP
			//	if !dstIP.Equal(selfIP) {
			//		reply, err := Ping(dstIP.String(), packet.Data()[20:])
			//		if err != nil {
			//			log.Warn(err)
			//		} else {
			//			err := SendIP(d, dstIP, ipv4.SrcIP, reply[20:])
			//			if err != nil {
			//				log.Warn(err)
			//			}
			//		}
			//	}
			//}
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
					data := nfd.GetIPData(selfIP, []byte{180, 101, 50, 242}, nfd.GetMsg())
					_, err := d.Write(data)
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

	defer log.Sync()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	device := StartDevice(ctx)

	go listenAndPrint(ctx, device)

	message := test_data

	bufs := make([][]byte, BatchSize)
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{}

	gopacket.SerializeLayers(buf, opts,
		&layers.IPv4{SrcIP: net.IPv4(192, 168, 101, 1), DstIP: net.IPv4(192, 168, 101, 2)},
		&layers.UDP{SrcPort: 2333, DstPort: 2333},
		gopacket.Payload(message))

	for i := 0; i < BatchSize; i++ {
		bufs[i] = buf.Bytes()
		bufs[i][0] = (4 << 4)
	}

	time.Sleep(3 * time.Second)
	log.Info("start send test")

	for i := 0; i < BatchSize; i++ {
		_, err := device.Write(bufs[i])
		if err != nil {
			log.Error(err)
		}
	}

	time.Sleep(3 * time.Second)

	stop()
	<-ctx.Done()
}

//func main() {
//
//	defer log.Sync()
//
//	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
//	defer stop()
//
//	device := StartDevice(ctx)
//
//	go listenAndPrint(ctx, device)
//
//	time.Sleep(60 * time.Second)
//
//	stop()
//	<-ctx.Done()
//}
