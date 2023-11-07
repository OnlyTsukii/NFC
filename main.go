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
	BatchSize = 1
	BufSize   = 1024
)

var configCh = make(chan nfd.ConfigInfo)
var deviceCh = make(chan nfd.DeviceInfo)
var strategy = nfd.Strategy{BatchSize: BatchSize}
var log slog.Logger
var err error

func StartDeviceA(ctx context.Context) *nfd.NearFieldDevice {
	n := nfd.NewNearFieldDevice("192.168.101.1", "")
	n.Init(strategy)
	n.Run(ctx, configCh, deviceCh)
	return n
}

func StartDeviceB(ctx context.Context) *nfd.NearFieldDevice {
	n := nfd.NewNearFieldDevice("192.168.101.2", "")
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
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		size, err := d.Read(buf)
		if err != nil {
			log.Error(err)
		}
		if size > 0 {
			log.Info(buf)
		}
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

	gopacket.SerializeLayers(buf, opts,
		&layers.IPv4{SrcIP: net.IPv4(192, 168, 101, 1), DstIP: net.IPv4(192, 168, 101, 2)},
		&layers.UDP{SrcPort: 2333, DstPort: 2333},
		gopacket.Payload(messageA))
	data := buf.Bytes()
	data[0] = (4 << 4)
	n, err := deviceA.Write(data)
	if err != nil {
		log.Error(err)
	}
	log.Infof("send %d message success\n", n)

	time.Sleep(5 * time.Second)

	gopacket.SerializeLayers(buf, opts,
		&layers.IPv4{SrcIP: net.IPv4(192, 168, 101, 2), DstIP: net.IPv4(192, 168, 101, 1)},
		&layers.UDP{SrcPort: 2333, DstPort: 2333},
		gopacket.Payload(messageB))
	data = buf.Bytes()
	data[0] = (4 << 4)
	n, err = deviceB.Write(data)
	if err != nil {
		log.Error(err)
	}
	log.Infof("send %d message success\n", n)
	time.Sleep(5 * time.Second)

	stop()

	<-ctx.Done()

}
