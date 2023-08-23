package main

import (
	"context"
	"gitee.com/ccl0924/nfd/nfd"
	"net"
	"os/signal"
	"syscall"
	"time"

	slog "gitee.com/czy_hit/log"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

const (
	BatchSize = 1
	BufSize   = 1024
)

var configCh = make(chan nfd.ConfigInfo)
var deviceCh = make(chan nfd.DeviceInfo)
var strategy = nfd.Strategy{BatchSize: BatchSize}
var log slog.Logger
var err error

func StartDeviceA(ctx context.Context) *nfd.NearFieldDevice {
	n := nfd.NewNearFieldDevice("192.168.101.1", "", "0013a20041bb76a4", "localhost", 8001)
	n.Init(strategy)
	n.Run(ctx, configCh, deviceCh)
	return n
}

func StartDeviceB(ctx context.Context) *nfd.NearFieldDevice {
	n := nfd.NewNearFieldDevice("192.168.101.2", "", "0013a20041bb7684", "localhost", 8002)
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
	bufs := make([][]byte, BatchSize)
	buf := make([]byte, BufSize)
	bufs[0] = buf
	size := make([]int, BatchSize)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		d.Read(bufs, size, 0)
		if size[0] > 0 {
			log.Info(buf)
		}
		size[0] = 0
	}

}

func main() {
	defer log.Sync()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	deviceA := StartDeviceA(ctx)
	deviceB := StartDeviceB(ctx)
	messageA := []byte("message from A")
	messageB := []byte("message from B")

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{}

	go listenAndPrint(ctx, deviceA)
	go listenAndPrint(ctx, deviceB)

	time.Sleep(5 * time.Second)
	log.Info("start send test")

	bufs := make([][]byte, BatchSize)

	gopacket.SerializeLayers(buf, opts,
		&layers.IPv4{SrcIP: net.IPv4(192, 168, 101, 1), DstIP: net.IPv4(192, 168, 101, 2)},
		&layers.UDP{SrcPort: 2333, DstPort: 2333},
		gopacket.Payload(messageA))
	bufs[0] = buf.Bytes()
	bufs[0][0] = (4 << 4)
	log.Info(bufs[0])
	n, err := deviceA.Write(bufs, 0)
	if err != nil {
		log.Error(err)
	}
	log.Infof("send %d message success\n", n)

	time.Sleep(5 * time.Second)

	gopacket.SerializeLayers(buf, opts,
		&layers.IPv4{DstIP: net.IPv4(192, 168, 101, 1), SrcIP: net.IPv4(192, 168, 101, 2)},
		&layers.UDP{SrcPort: 2333, DstPort: 2333},
		gopacket.Payload(messageB))
	bufs[0] = buf.Bytes()
	bufs[0][0] = (4 << 4)
	n, err = deviceB.Write(bufs, 0)
	if err != nil {
		log.Error(err)
	}
	log.Infof("send %d message success\n", n)
	time.Sleep(5 * time.Second)
	stop()

	<-ctx.Done()

}
