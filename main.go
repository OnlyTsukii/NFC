package main

import (
	"ccl/go/nfd"
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"gitee.com/czy_hit/log"
)

func StartDevice(ctx context.Context) {

	strategy := nfd.Strategy{0}
	configCh := make(chan nfd.ConfigInfo)
	deviceCh := make(chan nfd.DeviceInfo)
	n := nfd.NewNearFieldDevice("192.168.0.1", "", "0013a20041bb76a4", "localhost", 8001)
	n.Init(strategy)
	n.Run(ctx, configCh, deviceCh)
	// time.Sleep(3 * time.Second)
	// configCh <- nfd.ConfigInfo{nfd.SERACH_NODES_REQ, 0, ""}
	// fmt.Println(<-deviceCh)
	// time.Sleep(3 * time.Second)
	// for i := 0; i < 20; i++ {
	// 	n.Write([][]byte{data}, 0)
	// }
}

func main() {
	_, err := log.NewLogger()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err != nil {
		fmt.Printf("%v\n", err)
	}

	data = data[:2048]

	go StartDevice(ctx)

	select {
	case <-ctx.Done():
	}
}
