package main

import (
	"ccl/go/log"
	"ccl/go/nfd"
	"context"
	"fmt"
	"time"
)

func StartDevice() {
	ctx, _ := context.WithCancel(context.Background())
	strategy := nfd.Strategy{0}
	configCh := make(chan nfd.ConfigInfo)
	deviceCh := make(chan nfd.DeviceInfo)
	n := nfd.NewNearFieldDevice("192.168.0.1", "", "0013a20041bb76a4", "localhost", 8001)
	n.Init(strategy)
	n.Run(ctx, configCh, deviceCh)
	// time.Sleep(3 * time.Second)
	// configCh <- nfd.ConfigInfo{nfd.SERACH_NODES_REQ, 0, ""}
	// fmt.Println(<-deviceCh)
	time.Sleep(5 * time.Second)
	for i := 0; i < 20; i++ {
		n.Write([][]byte{data}, 0)
	}
}

func main() {
	_, err := log.NewLogger()
	if err != nil {
		fmt.Printf("%v\n", err)
	}

	data = data[:2048]

	go StartDevice()

	for {
	}
}
