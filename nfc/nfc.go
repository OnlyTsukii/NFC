package nfc

import (
	"ccl/go/nfc/xbee"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.bug.st/serial/enumerator"
)

const (
	BCST      = 0
	ADDR_REQ  = 1
	ADDR_RESP = 2
	P2P       = 3
	RT_REQ    = 4
	RT_RESP   = 5
	ACK       = 6

	TRANSMISSION_TIMEOUT = 10

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

type PendingData struct {
	Seq  int
	Data []byte
}

type TimerData struct {
	Seq     int
	Timeout int
	Packet  Packet
}

type NFC struct {
	DevType  string
	DevPort  string
	Device   Device
	Mac      string
	Seq      int
	NextSeq  map[string]int
	IP       string
	Pending  map[string][]PendingData
	TxMap    map[int]Packet
	RxMap    map[string][]Packet
	RtMap    map[string][]int
	RxQueue  chan Packet
	RtQueue  chan Packet
	RxDataCh chan []byte
	SeqMap   map[string]int
	Sending  bool
	Started  bool
	TimerMap map[int][]TimerData
	Timeout  int
	stopCh   chan struct{}

	wg sync.WaitGroup

	Mutex  sync.Mutex
	Mutex1 sync.Mutex
	Mutex2 sync.Mutex
	Mutex3 sync.Mutex
}

func NewNFC(ip string) *NFC {
	return &NFC{
		IP:       ip,
		Seq:      -1,
		Pending:  make(map[string][]PendingData),
		TxMap:    make(map[int]Packet),
		RxMap:    make(map[string][]Packet),
		RxQueue:  make(chan Packet, 20),
		RtQueue:  make(chan Packet, 20),
		RtMap:    make(map[string][]int),
		RxDataCh: make(chan []byte, 64),
		SeqMap:   make(map[string]int),
		NextSeq:  make(map[string]int),
		TimerMap: make(map[int][]TimerData),
		stopCh:   make(chan struct{}),
		Started:  false,
		Timeout:  TRANSMISSION_TIMEOUT,
	}
}

func (n *NFC) DevIDF() bool {
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
	if n.DevIDF() {
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

func (n *NFC) Send(data []byte, destIP string) bool {
	res := false
	n.Seq = (n.Seq + 1) % 256

	if destIP == BCST_IP {
		p := NewPacket(n.Seq, BCST, n.Mac, BCST_ADDR, n.IP, destIP, data)
		n.Mutex.Lock()
		n.TxMap[n.Seq] = *p
		n.Mutex.Unlock()
		fmt.Printf("INFO: send BCST %v\n", p)
		res = n.Device.SendPacket(p.Encode(), BCST_ADDR)
	} else {
		if destMac, ok := ADDR_LIST[destIP]; ok {
			p := NewPacket(n.Seq, P2P, n.Mac, destMac, n.IP, destIP, data)
			n.Mutex.Lock()
			n.TxMap[n.Seq] = *p
			n.Mutex.Unlock()
			fmt.Printf("INFO: send P2P %v\n", p)
			res = n.Device.SendPacket(p.Encode(), destMac)
		} else {
			p := NewPacket(n.Seq, ADDR_REQ, n.Mac, BCST_ADDR, n.IP, destIP, []byte([]byte("None")))
			n.Mutex1.Lock()
			if _, ok := n.Pending[destIP]; !ok {
				n.Pending[destIP] = make([]PendingData, 0)
				n.Pending[destIP] = append(n.Pending[destIP], PendingData{p.Seq, data})
				fmt.Printf("INFO: send ADDR_REQ %v\n", p)
				n.UpdateTimerMap(ADDR_REQ, 0, p, 0)
				res = n.Device.SendPacket(p.Encode(), BCST_ADDR)
			} else {
				n.Pending[destIP] = append(n.Pending[destIP], PendingData{p.Seq, data})
				res = true
			}
			n.Mutex1.Unlock()
		}
	}

	// n.Seq = (n.Seq + 1) % 256

	n.Mutex.Lock()
	if len(n.TxMap) == 32 {
		keysToRemove := []int{}
		for key := range n.TxMap {
			keysToRemove = append(keysToRemove, key)
			if len(keysToRemove) == 16 {
				break
			}
		}
		for _, keyToRemove := range keysToRemove {
			delete(n.TxMap, keyToRemove)
		}
		// fmt.Println("clear txmap", len(n.TxMap))
	}
	n.Mutex.Unlock()

	return res
}

func (n *NFC) SendPendingData(destIP string) {
	n.Mutex1.Lock()
	packets := n.Pending[destIP]
	destMac := ADDR_LIST[destIP]
	delete(n.Pending, destIP)
	n.Mutex1.Unlock()
	for _, pd := range packets {
		p := NewPacket(pd.Seq, P2P, n.Mac, destMac, n.IP, destIP, pd.Data)
		n.Mutex.Lock()
		n.TxMap[p.Seq] = *p
		n.Mutex.Unlock()
		fmt.Printf("INFO: send pending data %v\n", p)
		n.Device.SendPacket(p.Encode(), destMac)
	}
}

func (n *NFC) SendRtReq(seq int, destMac string, destIP string) {
	p := NewPacket(seq, RT_REQ, n.Mac, destMac, n.IP, destIP, []byte("None"))
	n.UpdateTimerMap(RT_REQ, 0, p, 0)
	n.Device.SendPacket(p.Encode(), p.DestMac)
}

func (n *NFC) Receiver() {
	defer n.wg.Done()
	for n.Started {
		data, err := n.Device.ReceivePacket()
		if err != nil {
			p, err := DecodePacket(data)
			if err == nil {
				p = NewPacket(p.Seq, RT_REQ, p.DestMac, p.SrcMac, p.DestIP, p.SrcIP, []byte("None"))
				n.RtQueue <- *p
			} else {
				continue
			}
		} else {
			p, err := DecodePacket(data)
			if err != nil {
				continue
			}
			n.RxQueue <- *p
		}
	}
}

func (n *NFC) PacketHandler() {
	defer n.wg.Done()
	for n.Started {
		select {
		case p := <-n.RxQueue:
			if p.PacketType == RT_REQ {
				p := NewPacket(p.Seq, RT_RESP, p.DestMac, p.SrcMac, p.DestIP, p.SrcIP, n.TxMap[p.Seq].Data)
				n.Device.SendPacket(p.Encode(), p.DestMac)
				fmt.Printf("INFO: received a RT_REQ packet for seq:[%d], send RT_RESP %v\n", p.Seq, p)
			} else if p.PacketType == RT_RESP {
				fmt.Printf("INFO: received a RT_RESP %v\n", p.String())
				n.UpdateTimerMap(RT_REQ, p.Seq, nil, 1)
				n.InsertAndSortPacket(p.SrcIP, p)
				n.SetNextSeq(p.SrcIP, p.SrcMac)
			} else if p.PacketType == ADDR_REQ {
				if ADDR_LIST[p.SrcIP] == "" {
					ADDR_LIST[p.SrcIP] = p.SrcMac
				}
				if n.IP == p.DestIP {
					p := NewPacket(p.Seq, ADDR_RESP, n.Mac, p.SrcMac, p.DestIP, p.SrcIP, []byte("None"))
					fmt.Printf("INFO: received a ADDR_REQ packet, send ADDR_RESP %v\n", p)
					n.Device.SendPacket(p.Encode(), p.DestMac)
				} else {
					continue
				}
			} else if p.PacketType == ADDR_RESP {
				fmt.Printf("INFO: received a ADDR_RESP packet, send all the pending data\n")
				ADDR_LIST[p.SrcIP] = p.SrcMac
				n.UpdateTimerMap(ADDR_REQ, p.Seq, nil, 1)
				n.SendPendingData(p.SrcIP)
			} else if n.NextSeq[p.SrcMac] != p.Seq {
				seq := n.NextSeq[p.SrcMac]
				cntu := false
				for v := range n.RtMap[p.SrcMac] {
					if v == seq {
						cntu = true
						break
					}
				}
				n.InsertAndSortPacket(p.SrcIP, p)
				n.SetNextSeq(p.SrcIP, p.SrcMac)
				if !cntu {
					fmt.Printf("INFO: received a disorder packet, send RT_REQ %v\n", p)
					n.RtMap[p.SrcMac] = append(n.RtMap[p.SrcMac], p.Seq)
					n.SendRtReq(n.NextSeq[p.SrcMac], p.SrcMac, p.SrcIP)
					continue
				}
				fmt.Printf("INFO: received a disorder packet, add to RxMap %v\n", p)
			} else {
				n.InsertAndSortPacket(p.SrcIP, p)
				n.SetNextSeq(p.SrcIP, p.SrcMac)
				if p.PacketType == P2P {
					fmt.Printf("INFO: received a P2P packet %v\n", p.String())
				} else {
					fmt.Printf("INFO: received a BCST packet %v\n", p.String())
				}
			}
		case <-n.stopCh:
			return
		}
	}
}

func (n *NFC) Timer() {
	defer n.wg.Done()
	for n.Started {
		for key := range n.TimerMap {
			for i := range n.TimerMap[key] {
				n.TimerMap[key][i].Timeout -= 1
				if n.TimerMap[key][i].Timeout == 0 {
					n.RtQueue <- n.TimerMap[key][i].Packet
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
}

func (n *NFC) Retransmitter() {
	defer n.wg.Done()
	for n.Started {
		select {
		case p := <-n.RtQueue:
			n.UpdateTimerMap(p.PacketType, p.Seq, nil, 2)
			fmt.Printf("INFO: send rt %v\n", p.String())
			if p.DestMac == BCST_ADDR {
				n.Device.SendPacket(p.Encode(), BCST_ADDR)
			} else {
				n.Device.SendPacket(p.Encode(), p.DestMac)
			}
		case <-n.stopCh:
			return
		}
	}
}

func (n *NFC) PushData() {
	defer n.wg.Done()
	for n.Started {
		for key := range n.RxMap {
			for _, v := range n.RxMap[key] {
				if n.SeqMap[key] == v.Seq {
					n.RxDataCh <- v.Data
					n.SeqMap[key] = (n.SeqMap[key] + 1) % 256
				} else {
					break
				}
			}
		}
	}
}

func (n *NFC) SetNextSeq(srcIP string, srcMac string) {
	n.Mutex2.Lock()
	defer n.Mutex2.Unlock()

	next := 0
	for _, v := range n.RxMap[srcIP] {
		if next == v.Seq {
			next = (next + 1) % 256
		} else {
			break
		}
	}
	n.NextSeq[srcMac] = next
}

func (n *NFC) InsertAndSortPacket(key string, packet Packet) {
	n.Mutex2.Lock()
	defer n.Mutex2.Unlock()

	packets := n.RxMap[key]
	if packets == nil {
		n.RxMap[key] = []Packet{packet}
		return
	}
	for _, v := range packets {
		if v.Seq == packet.Seq {
			return
		}
	}
	n.RxMap[key] = append(packets, packet)
	sort.Slice(n.RxMap[key], func(i, j int) bool {
		return n.RxMap[key][i].Seq < n.RxMap[key][j].Seq
	})
	for _, v := range n.RxMap[key] {
		fmt.Print(v.Seq)
	}
	fmt.Println()
}

func (n *NFC) UpdateTimerMap(key int, seq int, packet *Packet, op int) {
	n.Mutex3.Lock()
	defer n.Mutex3.Unlock()
	if op == 0 {
		fmt.Printf("INFO: add element [%d][%d] to timer_map\n", key, packet.Seq)
		if n.TimerMap[key] == nil {
			n.TimerMap[key] = []TimerData{}
		}
		n.TimerMap[key] = append(n.TimerMap[key], TimerData{packet.Seq, n.Timeout, *packet})
		return
	}
	index := -1
	for i := range n.TimerMap[key] {
		if n.TimerMap[key][i].Seq == seq {
			index = i
			break
		}
	}
	if index == -1 {
		return
	}
	if op == 1 {
		fmt.Printf("INFO: remove element [%d][%d] from timer_map\n", key, n.TimerMap[key][index].Seq)
		n.TimerMap[key] = append(n.TimerMap[key][:index], n.TimerMap[key][index+1:]...)
	} else if op == 2 {
		fmt.Printf("INFO: update element [%d][%d]\n", key, n.TimerMap[key][index].Seq)
		n.TimerMap[key][index].Timeout = n.Timeout
	}
}

func (n *NFC) Start() {
	n.stopCh = make(chan struct{})
	n.Started = true
	n.Device.Start()
	n.wg.Add(5)
	go n.Receiver()
	go n.PacketHandler()
	go n.Timer()
	go n.Retransmitter()
	go n.PushData()
}

func (n *NFC) Stop() {
	n.Device.Stop()
	n.Started = false
	close(n.stopCh)
	n.wg.Wait()
}

func (n *NFC) Close() {
	n.Device.Close()
	n.Started = false
	close(n.stopCh)
	n.wg.Wait()
}
