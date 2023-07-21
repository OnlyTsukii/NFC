package main

import (
	"ccl/go/nfc"
	"fmt"
	"os"
)

func main() {
	file, err := os.Open("8KB.txt")
	if err != nil {
		fmt.Printf("Open file failed：%s\n", err)
		return
	}
	defer file.Close()
	data := make([]byte, 5000)

	n, err := file.Read(data)
	if err != nil {
		fmt.Printf("Read file failed：%s\n", err)
	}
	data = data[:n]

	nfc := nfc.NewNFC("192.168.0.1")
	nfc.Open()
	nfc.Start()
	nfc.Send(data, "192.168.0.2")
	for {
	}
}
