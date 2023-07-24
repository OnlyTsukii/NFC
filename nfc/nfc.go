package nfc

import (
	"ccl/go/nfc/xbee"
	"fmt"
	"sync"
	"time"

	"go.bug.st/serial/enumerator"
)

const (
	BCST      = 0
	ADDR_REQ  = 1
	ADDR_RESP = 2
	P2P       = 3
	ACK       = 4

	TRANSMISSION_TIMEOUT = 15

	BCST_ADDR = "000000000000FFFF"
	BCST_IP   = "255.255.255.255"
)

var (
	DEVLIST = map[string]string{
		"2FE3:0100": "Bluetooth",
		"0403:6001": "Xbee",
	}

	ADDR_LIST = map[string]string{}
)

type Device interface {
	SendPacket(data []byte, remoteAddr string) bool
	ReceivePacket() ([]byte, error)
	Start()
	Stop()
	Close()
}

type TimerData struct {
	Packet  Packet
	Timeout int
}

type TxData struct {
	data   []byte
	destIP string
}

type NFC struct {
	DevType     string
	DevPort     string
	Device      Device
	Mac         string
	Seq         int
	IP          string
	RxMap       map[string][]Packet
	RtMap       map[string][]int
	Txqueue     chan TxData
	AckQueue    chan Packet
	DataQueue   chan Packet
	RxDataCh    chan []byte
	Started     bool
	TimerPacket TimerData
	stopCh      chan struct{}

	wg     sync.WaitGroup
	Mutex  sync.Mutex
	Mutex2 sync.Mutex
}

func NewNFC(ip string) *NFC {
	return &NFC{
		IP:        ip,
		Seq:       -1,
		RxMap:     make(map[string][]Packet),
		Txqueue:   make(chan TxData, 128),
		DataQueue: make(chan Packet, 3),
		AckQueue:  make(chan Packet, 3),
		RtMap:     make(map[string][]int),
		RxDataCh:  make(chan []byte, 128),
		stopCh:    make(chan struct{}),
		Started:   false,
	}
}

func DevIDF(n *NFC) bool {
	portList, _ := enumerator.GetDetailedPortsList()
	for _, port := range portList {
		id := fmt.Sprintf("%s:%s", port.VID, port.PID)
		if devType, ok := DEVLIST[id]; ok {
			n.DevType = devType
			n.DevPort = port.Name
			return true
		}
	}
	return false
}

func (n *NFC) Open() bool {
	if DevIDF(n) {
		switch n.DevType {
		case "Xbee":
			xbee, err := xbee.NewXbee(n.DevPort, 115200)
			if err != nil {
				fmt.Printf("Error occurred when opening the serial port: %v \n", err)
				return false
			}
			n.Mac = xbee.Mac
			var dev Device = xbee
			n.Device = dev
		case "Bluetooth":
			// Bluetooth device initialization
		}
		return true
	} else {
		fmt.Println("Failed to identify the device type")
		return false
	}
}

func (n *NFC) Send(data []byte, destIP string) {
	n.Txqueue <- TxData{data, destIP}
}

func Sender(n *NFC) {
	defer n.wg.Done()

	for n.Started {
		select {
		case tx := <-n.Txqueue:
			n.Seq = (n.Seq + 1) % 256

			if tx.destIP == BCST_IP {
				p := NewPacket(n.Seq, BCST, n.Mac, BCST_ADDR, n.IP, tx.destIP, tx.data)
				n.Device.SendPacket(p.Encode(), BCST_ADDR)
				fmt.Printf("INFO: send BCST %v\n", p.String())
			} else {
				if destMac, ok := ADDR_LIST[tx.destIP]; ok {
					p := NewPacket(n.Seq, P2P, n.Mac, destMac, n.IP, tx.destIP, tx.data)
					n.Device.SendPacket(p.Encode(), destMac)
					fmt.Printf("INFO: send P2P %v\n", p.String())
					n.Mutex.Lock()
					n.TimerPacket = TimerData{*p, TRANSMISSION_TIMEOUT}
					n.Mutex.Unlock()
					WaitForAck(n, p)
				} else {
					p := NewPacket(n.Seq, ADDR_REQ, n.Mac, BCST_ADDR, n.IP, tx.destIP, []byte([]byte("None")))
					n.Device.SendPacket(p.Encode(), BCST_ADDR)
					fmt.Printf("INFO: send ADDR_REQ %v\n", p.String())
					n.Mutex.Lock()
					n.TimerPacket = TimerData{*p, TRANSMISSION_TIMEOUT}
					n.Mutex.Unlock()
					WaitForAck(n, p)

					p = NewPacket(n.Seq, P2P, n.Mac, ADDR_LIST[tx.destIP], n.IP, tx.destIP, tx.data)
					n.Device.SendPacket(p.Encode(), p.DestMac)
					fmt.Printf("INFO: send P2P %v\n", p.String())
					n.Mutex.Lock()
					n.TimerPacket = TimerData{*p, TRANSMISSION_TIMEOUT}
					n.Mutex.Unlock()
					WaitForAck(n, p)
				}
			}
		case <-n.stopCh:
			return
		}
	}
}

var (
	before = -1
	count  = 0
)

func WaitForAck(n *NFC, p *Packet) {
	for {
		ack := <-n.AckQueue
		if ack.Seq == p.Seq {
			if ack.PacketType == ACK {
				fmt.Printf("INFO: received a ACK for [%v]\n", p.Seq)
				if ack.Seq != before {
					count++
					fmt.Println("count:", count)
					before = ack.Seq
				}
			} else {
				fmt.Printf("INFO: received a ADDR_RESP for [%v]\n", p.Seq)
				ADDR_LIST[ack.SrcIP] = ack.SrcMac
			}
			n.Mutex.Lock()
			n.TimerPacket.Timeout = 2147483647
			n.Mutex.Unlock()
			break
		}
	}
}

func Receiver(n *NFC) {
	defer n.wg.Done()
	for n.Started {
		data, err := n.Device.ReceivePacket()
		if err == nil {
			p, err := DecodePacket(data)
			if err != nil {
				continue
			}
			if p.PacketType == ACK || p.PacketType == ADDR_RESP {
				n.AckQueue <- *p
			} else {
				n.DataQueue <- *p
			}
		}
	}
}

func PacketHandler(n *NFC) {
	defer n.wg.Done()
	for n.Started {
		select {
		case p := <-n.DataQueue:
			if p.PacketType == ADDR_REQ {
				if ADDR_LIST[p.SrcIP] == "" {
					ADDR_LIST[p.SrcIP] = p.SrcMac
				}
				if n.IP == p.DestIP {
					p := NewPacket(p.Seq, ADDR_RESP, n.Mac, p.SrcMac, p.DestIP, p.SrcIP, []byte("None"))
					n.Device.SendPacket(p.Encode(), p.DestMac)
					fmt.Printf("INFO: received a ADDR_REQ packet, send ADDR_RESP %v\n", p)
				} else {
					continue
				}
			} else {
				InsertPacket(n, p.SrcIP, p)
				if p.PacketType == P2P {
					p := NewPacket(p.Seq, ACK, n.Mac, p.SrcMac, n.IP, p.SrcIP, []byte("None"))
					// fmt.Printf("INFO: received a P2P, send ACK %v\n", p.String())
					n.Device.SendPacket(p.Encode(), p.DestMac)
				} else {
					fmt.Printf("INFO: received a BCST %v\n", p.String())
				}
			}
		case <-n.stopCh:
			return
		}
	}
}

func Timer(n *NFC) {
	defer n.wg.Done()
	for n.Started {
		n.TimerPacket.Timeout--
		if n.TimerPacket.Timeout == 0 {
			p := n.TimerPacket.Packet
			n.Device.SendPacket(p.Encode(), p.DestMac)
			fmt.Printf("INFO: send retransmission %v\n", p.String())
			n.TimerPacket.Timeout = TRANSMISSION_TIMEOUT
		}
		time.Sleep(1 * time.Second)
	}
}

func PushData(n *NFC) {
	defer n.wg.Done()
	for n.Started {
		for key := range n.RxMap {
			for _, v := range n.RxMap[key] {
				n.RxDataCh <- v.Data
			}
			delete(n.RxMap, key)
		}
		time.Sleep(1 * time.Second)
	}
}

func (n *NFC) GetData() []byte {
	select {
	case data := <-n.RxDataCh:
		return data
	default:
		return nil
	}
}

func InsertPacket(n *NFC, key string, packet Packet) {
	n.Mutex2.Lock()
	defer n.Mutex2.Unlock()

	packets := n.RxMap[key]
	if packets == nil {
		n.RxMap[key] = []Packet{packet}
	}
	for _, v := range packets {
		if v.Seq == packet.Seq {
			return
		}
	}
	n.RxMap[key] = append(packets, packet)
}

func (n *NFC) Start() bool {
	if !n.Open() {
		return false
	}
	n.stopCh = make(chan struct{})
	n.Started = true
	n.Device.Start()
	n.wg.Add(5)
	go Receiver(n)
	go PushData(n)
	go PacketHandler(n)
	go Sender(n)
	go Timer(n)
	return true
}

func (n *NFC) Stop() {
	n.Device.Stop()
	n.Started = false
	close(n.stopCh)
	n.wg.Wait()
}

func (n *NFC) Destroy() {
	n.Device.Close()
	n.Started = false
	close(n.stopCh)
	n.wg.Wait()
}
