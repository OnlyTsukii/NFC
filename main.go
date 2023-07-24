package main

import (
	"ccl/go/nfc"
	"fmt"
	"os"
	"time"
)

func main() {
	file, err := os.Open("8KB.txt")
	if err != nil {
		fmt.Printf("Open file failed：%s\n", err)
		return
	}
	defer file.Close()
	data := make([]byte, 2048)

	n, err := file.Read(data)
	if err != nil {
		fmt.Printf("Read file failed：%s\n", err)
	}
	data = data[:n]

	nfc := nfc.NewNFC("192.168.0.1")
	nfc.Open()
	nfc.Start()

	count := 1
	time.Sleep(8 * time.Second)
	for i := 0; i < 20; i++ {
		// nfc.Send(data, "192.168.0.2")
		if count%4 == 0 {
			nfc.Send(data, "255.255.255.255")
		} else {
			nfc.Send(data, "192.168.0.2")
		}
		count++
	}
	for {
		// data := nfc.GetData()
		// if data != nil {
		// 	fmt.Println(len(data))
		// }
		// time.Sleep(3 * time.Second)
	}
}
