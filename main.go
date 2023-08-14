package main

import (
	"ccl/go/nfd"
	"context"
	"fmt"
	"time"
)

func main() {
	data = data[:2048]

	ctx, _ := context.WithCancel(context.Background())
	strategy := nfd.Strategy{0}
	configCh := make(chan nfd.ConfigInfo)
	deviceCh := make(chan nfd.DeviceInfo)
	n := nfd.NewNearFieldDevice("192.168.0.1", "")
	n.Init(strategy)
	n.Run(ctx, configCh, deviceCh)
	time.Sleep(3 * time.Second)
	fmt.Println("---------------------------")
	n.Stop()
	// for i := 0; i < 20; i++ {
	// 	n.Write([][]byte{data}, 0)
	// }
	for {
	}
}
