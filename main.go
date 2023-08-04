package main

import (
	"ccl/go/nfc"
	"context"
	"time"
)

func main() {
	data = data[:1024]

	ctx, _ := context.WithCancel(context.Background())
	strategy := nfc.Strategy{0}
	n := nfc.NewNFC("192.168.0.1", "")
	n.Init(strategy)
	n.Run(ctx)
	// count := 1
	// for i := 0; i < 20; i++ {
	// 	n.Write([][]byte{data}, 0)
	// 	// if count%4 == 0 {
	// 	// 	nfc.Write(data, "255.255.255.255")
	// 	// } else {
	// 	// 	nfc.Write(data, "192.168.0.2")
	// 	// }
	// 	count++
	// }
	time.Sleep(3 * time.Second)
	nfc.GetNodes(n)
	nfc.SendICMP(n, GetMsg())
	// for i := 0; i < 10; i++ {
	// 	time.Sleep(3 * time.Second)
	// 	nfc.GetNodes(n)
	// 	nfc.SendICMP(n, GetMsg())
	// }
	for {
		// num := runtime.NumGoroutine()
		// fmt.Println("num of goroutine:", num)
		// time.Sleep(3 * time.Second)
	}
}

func GetMsg() []byte {
	// 构造一个简单的ICMP消息
	msg := make([]byte, 48)
	msg[0] = 8  // Type: 8 (Echo Request)
	msg[1] = 0  // Code: 0
	msg[2] = 0  // Checksum (placeholder)
	msg[3] = 0  // Checksum (placeholder)
	msg[4] = 0  // Identifier (arbitrary)
	msg[5] = 13 // Identifier (arbitrary)
	msg[6] = 0  // Sequence Number (arbitrary)
	msg[7] = 37 // Sequence Number (arbitrary)

	// 计算校验和
	checksum := checkSum(msg)
	msg[2] = byte(checksum >> 8)
	msg[3] = byte(checksum)

	return msg
}

// 计算校验和
func checkSum(msg []byte) uint16 {
	sum := 0
	for i := 0; i < len(msg)-1; i += 2 {
		sum += int(msg[i])*256 + int(msg[i+1])
	}
	sum = (sum >> 16) + (sum & 0xffff)
	sum += sum >> 16
	return uint16(^sum)
}
