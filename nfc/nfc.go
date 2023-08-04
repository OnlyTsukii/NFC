package nfc

import (
	"ccl/go/nfc/xbee"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.bug.st/serial/enumerator"
)

const (
	BCST         = 0
	ADDR_REQ     = 1
	ADDR_RESP    = 2
	P2P          = 3
	ACK          = 4
	SCAN_REQ     = 5
	SCAN_RESP    = 6
	CONNECT_REQ  = 7
	CONNECT_RESP = 8

	XBEE      = 0
	BLUETOOTH = 1
	UNKNOWN   = 2

	TRANSMISSION_TIMEOUT = 8
	MIN_RSSI             = -128
	MAX_RETRIES          = 3

	BCST_IP  = "255.255.255.255"
	BCST_MAC = "000000000000FFFF"

	DEFAULT_MAC  = "FFFFFFFFFFFFFFFF"
	DEFAULT_IPv4 = "192.168.0.1"
	DEFAULT_IPv6 = "1111:2222:3333:4444:5555:6666:7777:8888"
)

var (
	DEV_LIST = map[string]int{
		"2FE3:0100": BLUETOOTH,
		"0403:6001": XBEE,
	}

	ADDR_LIST = map[string]string{}

	child_ctx, cancel = context.WithCancel(context.TODO())
)

type Strategy struct {
	BatchSize int
}

type ConfigInfo struct {
}

type DeviceInfo struct {
}

type Device interface {
	SendPacket(data []byte, remoteAddr string)
	ReceivePacket() ([]byte, error)
	GetNodes() ([]string, error)
	Start(ctx context.Context)
	Stop()
	Close()
}

type TimerData struct {
	Packet  Packet
	Timeout int
	Retries int
}

type TxData struct {
	Data    []byte
	DestIP  string
	DestMac string
	TxType  int
}

type DevDesc struct {
	DevType int
	DevPort string
	Mac     string
	RSSI    int
	Device  Device
}

type NFC struct {
	DevDesc     DevDesc
	Seq         int
	IPv4        string
	IPv6        string
	Nodes       map[string]int
	TxQueue     chan TxData
	AckQueue    chan Packet
	DataQueue   chan Packet
	ScanQueue   chan Packet
	RxDataCh    chan []byte
	TimerPacket TimerData
	strategy    Strategy
	running     bool

	Mutex  sync.Mutex
	Mutex2 sync.Mutex
	Mutex3 sync.Mutex
	Mutex4 sync.Mutex
	WG     sync.WaitGroup
}

func NewNFC(ipv4 string, ipv6 string) *NFC {
	if ipv4 == "" {
		ipv4 = DEFAULT_IPv4
	}
	if ipv6 == "" {
		ipv6 = DEFAULT_IPv6
	}
	return &NFC{
		IPv4:      ipv4,
		IPv6:      ipv6,
		Seq:       -1,
		Nodes:     make(map[string]int),
		TxQueue:   make(chan TxData, 64),
		DataQueue: make(chan Packet, 3),
		AckQueue:  make(chan Packet, 3),
		ScanQueue: make(chan Packet, 32),
		RxDataCh:  make(chan []byte, 64),
	}
}

func (n *NFC) Open() error {
	if DevIdf(n) {
		switch n.DevDesc.DevType {
		case XBEE:
			xbee, err := xbee.NewXbee(n.DevDesc.DevPort, 115200)
			if err != nil {
				return err
			}
			n.DevDesc.Mac = xbee.Mac
			n.DevDesc.Device = xbee
		case BLUETOOTH:
			// Bluetooth device initialization
		}
	} else {
		return errors.New("failed to identify the device type")
	}
	return nil
}

func DevIdf(n *NFC) bool {
	portList, _ := enumerator.GetDetailedPortsList()
	for _, port := range portList {
		id := fmt.Sprintf("%s:%s", port.VID, port.PID)
		if devType, ok := DEV_LIST[id]; ok {
			n.DevDesc = DevDesc{devType, port.Name, DEFAULT_MAC, MIN_RSSI, nil}
			return true
		}
	}
	return false
}

var (
	before    = -1
	ack_count = 0
)

func WaitForAck(n *NFC, p *Packet) bool {
	for {
		select {
		case ack := <-n.AckQueue:
			if ack.Seq == p.Seq {
				if ack.PacketType == ACK {
					if ack.Seq != before {
						fmt.Printf("INFO: %d received a ACK for [%v]\n", time.Now().UnixMilli(), p.Seq)
						ack_count++
						fmt.Println("count:", ack_count)
						before = ack.Seq
					}
				} else if ack.PacketType == ADDR_RESP {
					srcIP, _, err := GetIP(ack.Data)
					if err == nil {
						ADDR_LIST[srcIP] = ack.SrcMac
					}
					fmt.Printf("INFO: %d received a ADDR_RESP for [%v]\n", time.Now().UnixMilli(), p.Seq)
				} else if ack.PacketType == SCAN_RESP {
					fmt.Printf("INFO: %d received a SCAN_RESP for [%v]\n", time.Now().UnixMilli(), p.Seq)
				}
				n.Mutex.Lock()
				n.TimerPacket.Timeout = 2147483647
				n.Mutex.Unlock()
				return true
			}
		default:
			if n.TimerPacket.Retries == 0 {
				fmt.Println("stop waiting for ack")
				return false
			}
		}
	}
}

func SetNextSeq(n *NFC) {
	n.Mutex4.Lock()
	defer n.Mutex4.Unlock()
	n.Seq = (n.Seq + 1) % 256
}

func Sender(ctx context.Context, n *NFC) {
	defer n.WG.Done()
	fmt.Println("sender started")
	for {
		select {
		case tx := <-n.TxQueue:

			SetNextSeq(n)
			srcMac := n.DevDesc.Mac

			if tx.TxType == BCST {
				p := NewPacket(n.Seq, BCST, srcMac, BCST_MAC, tx.Data)
				n.DevDesc.Device.SendPacket(p.Encode(), p.DestMac)
				fmt.Printf("INFO: %d send BCST %v\n", time.Now().UnixMilli(), p.String())
			} else {
				if destMac, ok := ADDR_LIST[tx.DestIP]; ok {
					p := NewPacket(n.Seq, P2P, srcMac, destMac, tx.Data)
					n.DevDesc.Device.SendPacket(p.Encode(), p.DestMac)
					fmt.Printf("INFO: %d send P2P %v\n", time.Now().UnixMilli(), p.String())
					n.Mutex.Lock()
					n.TimerPacket = TimerData{*p, TRANSMISSION_TIMEOUT, MAX_RETRIES}
					n.Mutex.Unlock()
					WaitForAck(n, p)
				} else {
					p := NewPacket(n.Seq, ADDR_REQ, srcMac, BCST_MAC, CreateIPData(n, tx.DestIP, nil))
					n.DevDesc.Device.SendPacket(p.Encode(), p.DestMac)
					fmt.Printf("INFO: %d send ADDR_REQ %v\n", time.Now().UnixMilli(), p.String())
					n.Mutex.Lock()
					n.TimerPacket = TimerData{*p, TRANSMISSION_TIMEOUT, MAX_RETRIES}
					n.Mutex.Unlock()
					if WaitForAck(n, p) {
						SetNextSeq(n)
						destMac = ADDR_LIST[tx.DestIP]
						p := NewPacket(n.Seq, P2P, srcMac, destMac, tx.Data)
						n.DevDesc.Device.SendPacket(p.Encode(), p.DestMac)
						fmt.Printf("INFO: %d send P2P %v\n", time.Now().UnixMilli(), p.String())
						n.Mutex.Lock()
						n.TimerPacket = TimerData{*p, TRANSMISSION_TIMEOUT, MAX_RETRIES}
						n.Mutex.Unlock()
						WaitForAck(n, p)
					}
				}
			}
		case <-ctx.Done():
			fmt.Println("sender stopped")
			return
		}
	}
}

func Receiver(ctx context.Context, n *NFC) {
	defer n.WG.Done()
	fmt.Println("receiver started")
	for {
		select {
		case <-ctx.Done():
			fmt.Println("receiver stopped")
			return
		default:
			data, err := n.DevDesc.Device.ReceivePacket()
			if err == nil {
				p, err := DecodePacket(data)
				if err == nil {
					if p.PacketType == ACK || p.PacketType == ADDR_RESP {
						n.AckQueue <- *p
					} else if p.PacketType == SCAN_RESP {
						n.AckQueue <- *p
						n.ScanQueue <- *p
					} else {
						n.DataQueue <- *p
					}
				}
			}
		}
	}
}

func PacketHandler(ctx context.Context, n *NFC) {
	defer n.WG.Done()
	fmt.Println("handler started")
	for {
		select {
		case p := <-n.DataQueue:
			SrcIP, DestIP, err := GetIP(p.Data)
			if err == nil {
				n.Mutex3.Lock()
				ADDR_LIST[SrcIP] = p.SrcMac
				n.Mutex3.Unlock()
				if p.PacketType == ADDR_REQ {
					if n.IPv4 == DestIP || n.IPv6 == DestIP {
						p := NewPacket(p.Seq, ADDR_RESP, n.DevDesc.Mac, p.SrcMac, CreateIPData(n, SrcIP, nil))
						n.DevDesc.Device.SendPacket(p.Encode(), p.DestMac)
						fmt.Printf("INFO: %d received a ADDR_REQ packet, send ADDR_RESP %v\n", time.Now().UnixMilli(), p)
					} else {
						continue
					}
				} else if p.PacketType == SCAN_REQ {
					// Create deviceInfo and add it to the DeviceInfo channel
					p := NewPacket(p.Seq, SCAN_RESP, n.DevDesc.Mac, p.SrcMac, CreateIPData(n, SrcIP, []byte{byte(1)}))
					n.DevDesc.Device.SendPacket(p.Encode(), p.DestMac)
					fmt.Printf("INFO: %d received a SCAN_REQ packet, send SCAN_RESP %v\n", time.Now().UnixMilli(), p)
				} else if p.PacketType == CONNECT_REQ {
					data, err := GetIPData(p.Data)
					if err != nil {
						fmt.Println(err)
					}
					fmt.Printf("INFO: %d received a CONNECT_REQ %v\n", time.Now().UnixMilli(), p.String())
					resp, err := Ping("122.51.216.252", data)
					if err == nil {
						p := NewPacket(p.Seq, CONNECT_RESP, n.DevDesc.Mac, p.SrcMac, CreateIPData(n, SrcIP, resp))
						n.DevDesc.Device.SendPacket(p.Encode(), p.DestMac)
						fmt.Printf("INFO: %d send CONNECT_RESP %v\n", time.Now().UnixMilli(), p.String())
					}
					// Create deviceInfo and add it to the DeviceInfo channel
				} else if p.PacketType == CONNECT_RESP {
					fmt.Printf("INFO: %d received a CONNECT_RESP %v\n", time.Now().UnixMilli(), p.String())
					// Create deviceInfo and add it to the DeviceInfo channel
				} else {
					n.RxDataCh <- p.Data
					if p.PacketType == P2P {
						p := NewPacket(p.Seq, ACK, n.DevDesc.Mac, p.SrcMac, CreateIPData(n, SrcIP, nil))
						// fmt.Printf("INFO: received a P2P, send ACK %v\n", p.String())
						n.DevDesc.Device.SendPacket(p.Encode(), p.DestMac)
					} else {
						fmt.Printf("INFO: %d received a BCST %v\n", time.Now().UnixMilli(), p.String())
					}
				}
			} else {
				fmt.Println("get ip address failed")
			}
		case <-ctx.Done():
			fmt.Println("handler stopped")
			return
		}
	}
}

func Timer(ctx context.Context, n *NFC) {
	defer n.WG.Done()
	fmt.Println("timer started")
	for {
		select {
		case <-ctx.Done():
			fmt.Println("timer stopped")
			return
		default:
			n.TimerPacket.Timeout--
			if n.TimerPacket.Timeout == 0 {
				if n.TimerPacket.Retries > 0 {
					n.TimerPacket.Retries--
					p := n.TimerPacket.Packet
					n.DevDesc.Device.SendPacket(p.Encode(), p.DestMac)
					fmt.Printf("INFO: %d send retransmission %v\n", time.Now().UnixMilli(), p.String())
					n.TimerPacket.Timeout = TRANSMISSION_TIMEOUT
				}
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func ConfigHandler(ctx context.Context, n *NFC, configCh chan ConfigInfo) {
	defer n.WG.Done()
	fmt.Println("config handler started")
	for {
		select {
		case <-ctx.Done():
			fmt.Println("config handler stopped")
		case config := <-configCh:
			fmt.Println(config)
			// Process config information
		default:
			// DO NOTHING
		}
	}
}

func GetNodes(n *NFC) error {
	fmt.Println("Start discovering nodes...")
	addrs, err := n.DevDesc.Device.GetNodes()
	if err != nil {
		return err
	}
	p := NewPacket(0, SCAN_REQ, n.DevDesc.Mac, DEFAULT_MAC, CreateIPData(n, DEFAULT_IPv4, nil))
	for i := range addrs {
		SetNextSeq(n)
		p.Seq = n.Seq
		p.DestMac = addrs[i]
		n.DevDesc.Device.SendPacket(p.Encode(), p.DestMac)
		fmt.Printf("INFO: %d send SCAN_REQ %v\n", time.Now().UnixMilli(), p.String())
		n.Mutex.Lock()
		n.TimerPacket = TimerData{*p, TRANSMISSION_TIMEOUT, MAX_RETRIES}
		n.Mutex.Unlock()
		WaitForAck(n, p)
	}
	for len(n.ScanQueue) > 0 {
		resp := <-n.ScanQueue
		n.Mutex3.Lock()
		SrcIP, _, err := GetIP(resp.Data)
		if err == nil {
			ADDR_LIST[SrcIP] = resp.SrcMac
		}
		n.Mutex3.Unlock()
		n.Nodes[SrcIP] = int(resp.Data[len(resp.Data)-1])
	}
	fmt.Println("Nodes status:", n.Nodes)
	return nil
}

func SendICMP(n *NFC, data []byte) error {
	destIP := ""
	for key := range n.Nodes {
		if n.Nodes[key] == 1 {
			destIP = key
		}
	}
	if destIP != "" {
		SetNextSeq(n)
		p := NewPacket(n.Seq, CONNECT_REQ, n.DevDesc.Mac, ADDR_LIST[destIP], CreateIPData(n, destIP, data))
		n.DevDesc.Device.SendPacket(p.Encode(), p.DestMac)
		fmt.Printf("INFO: %d send CONNECT_REQ %v\n", time.Now().UnixMilli(), p.String())
		return nil
	} else {
		fmt.Println("All nodes are offline")
		return errors.New("all nodes are offline")
	}
}

func (n *NFC) Init(s Strategy) error {
	fmt.Println("initializing...")
	n.strategy = s
	err := n.Open()
	if err != nil {
		fmt.Println("initialize failed")
		return err
	}
	fmt.Println("initialize finished")
	return nil
}

func Run(ctx context.Context, configInfo chan ConfigInfo, devInfo chan DeviceInfo) error {
	// TODO
	return nil
}

func (n *NFC) Run(ctx context.Context) error {
	child_ctx, cancel = context.WithCancel(ctx)
	n.DevDesc.Device.Start(child_ctx)
	n.WG.Add(4)
	go Receiver(child_ctx, n)
	go PacketHandler(child_ctx, n)
	go Sender(child_ctx, n)
	go Timer(child_ctx, n)
	// go ConfigHandler(child_ctx, n, configInfo)
	n.running = true
	fmt.Println("nfc started")
	return nil
}

func (n *NFC) Stop() error {
	n.DevDesc.Device.Stop()
	cancel()
	n.WG.Wait()
	fmt.Println("nfc stopped")
	n.running = false
	return nil
}

func (n *NFC) Read(bufs *[][]byte, sizes *[]int, offset int) (int, error) {
	for i := offset; i < len(*bufs); i++ {
		select {
		case data := <-n.RxDataCh:
			(*bufs)[i] = data
			(*sizes)[i] = len(data)
		default:
			return i - offset, errors.New("rx queue is empty")
		}
	}
	return len(*bufs) - offset, nil
}

func (n *NFC) Write(bufs [][]byte, offset int) (int, error) {
	for i := offset; i < len(bufs); i++ {
		_, destIP, err := GetIP(bufs[i])
		if err != nil {
			fmt.Println(err)
			return i - offset, err
		}
		tx_type := P2P
		if destIP == BCST_IP {
			tx_type = BCST
		}
		select {
		case n.TxQueue <- TxData{bufs[i], destIP, "", tx_type}:
		default:
			return i - offset, errors.New("tx queue is full")
		}
	}
	return len(bufs) - offset, nil
}

func (n *NFC) BatchSize() (int, error) {
	return n.strategy.BatchSize, nil
}
