package main

import (
	"ccl/go/nfd"
	"context"
)

func main() {
	data = data[:2048]

	ctx, _ := context.WithCancel(context.Background())
	strategy := nfd.Strategy{0}
	n := nfd.NewNearFieldDevice("192.168.0.1", "")
	n.Init(strategy)
	n.Run(ctx)
	for i := 0; i < 20; i++ {
		n.Write([][]byte{data}, 0)
	}
	for {
	}
}
