package nfd

import (
	"ccl/go/nfd/xbee"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.bug.st/serial/enumerator"
)

const (
	BCST        = 0
	P2P         = 1
	ACK         = 2
	ADDR_REQ    = 3
	ADDR_RESP   = 4
	STATUS_REQ  = 5
	STATUS_RESP = 6
	RELAY_REQ   = 7
	RELAY_RESP  = 8

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

type NearFieldDevice struct {
	DevDesc     DevDesc
	Seq         int
	IPv4        string
	IPv6        string
	Nodes       map[string]int
	TxQueue     chan TxData
	AckQueue    chan Packet
	DataQueue   chan Packet
	StatusQueue chan Packet
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

func NewNearFieldDevice(ipv4 string, ipv6 string) *NearFieldDevice {
	if ipv4 == "" {
		ipv4 = DEFAULT_IPv4
	}
	if ipv6 == "" {
		ipv6 = DEFAULT_IPv6
	}
	return &NearFieldDevice{
		IPv4:        ipv4,
		IPv6:        ipv6,
		Seq:         -1,
		Nodes:       make(map[string]int),
		TxQueue:     make(chan TxData, 64),
		DataQueue:   make(chan Packet, 3),
		AckQueue:    make(chan Packet, 3),
		StatusQueue: make(chan Packet, 32),
		RxDataCh:    make(chan []byte, 64),
	}
}

func DevIdf(n *NearFieldDevice) bool {
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

func (n *NearFieldDevice) Open() error {
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

var (
	before      = -1
	ack_count   = 0
	status_resp = 0
)

func WaitForAck(n *NearFieldDevice, p *Packet) bool {
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
				} else if ack.PacketType == STATUS_RESP {
					fmt.Printf("INFO: %d received a STATUS_RESP for [%v]\n", time.Now().UnixMilli(), p.Seq)
					status_resp++
				} else if ack.PacketType == RELAY_RESP {
					fmt.Printf("INFO: %d received a RELAY_RESP for [%v]\n", time.Now().UnixMilli(), p.Seq)
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

func SetNextSeq(n *NearFieldDevice) {
	n.Mutex4.Lock()
	defer n.Mutex4.Unlock()
	n.Seq = (n.Seq + 1) % 256
}

func Sender(ctx context.Context, n *NearFieldDevice) {
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
			} else if tx.TxType == STATUS_REQ {
				p := NewPacket(n.Seq, STATUS_REQ, srcMac, tx.DestMac, tx.Data)
				n.DevDesc.Device.SendPacket(p.Encode(), p.DestMac)
				fmt.Printf("INFO: %d send STATUS_REQ %v\n", time.Now().UnixMilli(), p.String())
				n.Mutex.Lock()
				n.TimerPacket = TimerData{*p, TRANSMISSION_TIMEOUT, MAX_RETRIES}
				n.Mutex.Unlock()
				WaitForAck(n, p)
			} else if tx.TxType == RELAY_REQ {
				p := NewPacket(n.Seq, RELAY_REQ, srcMac, tx.DestMac, tx.Data)
				n.DevDesc.Device.SendPacket(p.Encode(), p.DestMac)
				fmt.Printf("INFO: %d send RELAY_REQ %v\n", time.Now().UnixMilli(), p.String())
				n.Mutex.Lock()
				n.TimerPacket = TimerData{*p, TRANSMISSION_TIMEOUT, MAX_RETRIES}
				n.Mutex.Unlock()
				WaitForAck(n, p)
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

func Receiver(ctx context.Context, n *NearFieldDevice) {
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
					if p.PacketType == ACK || p.PacketType == ADDR_RESP || p.PacketType == RELAY_RESP {
						n.AckQueue <- *p
					} else if p.PacketType == STATUS_RESP {
						n.AckQueue <- *p
						n.StatusQueue <- *p
					} else {
						n.DataQueue <- *p
					}
				}
			}
		}
	}
}

func PacketHandler(ctx context.Context, n *NearFieldDevice) {
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
				} else if p.PacketType == STATUS_REQ {
					// Create deviceInfo and add it to the DeviceInfo channel
					p := NewPacket(p.Seq, STATUS_RESP, n.DevDesc.Mac, p.SrcMac, CreateIPData(n, SrcIP, []byte{byte(1)}))
					n.DevDesc.Device.SendPacket(p.Encode(), p.DestMac)
					fmt.Printf("INFO: %d received a STATUS_REQ packet, send STATUS_RESP %v\n", time.Now().UnixMilli(), p)
				} else if p.PacketType == RELAY_REQ {
					data, err := GetIPData(p.Data)
					if err != nil {
						fmt.Println(err)
					}
					fmt.Printf("INFO: %d received a RELAY_REQ %v\n", time.Now().UnixMilli(), p.String())
					resp, err := Ping(DestIP, data)
					if err == nil {
						p := NewPacket(p.Seq, RELAY_RESP, n.DevDesc.Mac, p.SrcMac, CreateIPData(n, SrcIP, resp))
						n.DevDesc.Device.SendPacket(p.Encode(), p.DestMac)
						fmt.Printf("INFO: %d send RELAY_RESP %v\n", time.Now().UnixMilli(), p.String())
					}
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

func Timer(ctx context.Context, n *NearFieldDevice) {
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
				} else if n.TimerPacket.Packet.PacketType == STATUS_REQ {
					status_resp = -1
				}
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func ConfigHandler(ctx context.Context, n *NearFieldDevice, configCh chan ConfigInfo) {
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

func NodesDetector(ctx context.Context, n *NearFieldDevice) {
	defer n.WG.Done()
	fmt.Println("nodes detector started")
	for {
		select {
		case <-ctx.Done():
			fmt.Println("nodes detector stopped")
			return
		default:
			fmt.Println("Start discovering nodes...")
			addrs, err := n.DevDesc.Device.GetNodes()
			fmt.Printf("Found %d nodes \n", len(addrs))
			if err == nil {
				for i := range addrs {
					n.TxQueue <- TxData{CreateIPData(n, DEFAULT_IPv4, nil), DEFAULT_IPv4, addrs[i], STATUS_REQ}
				}
				for {
					flag := false
					select {
					case resp := <-n.StatusQueue:
						n.Mutex3.Lock()
						SrcIP, _, err := GetIP(resp.Data)
						if err == nil {
							ADDR_LIST[SrcIP] = resp.SrcMac
						}
						n.Mutex3.Unlock()
						n.Nodes[SrcIP] = int(resp.Data[len(resp.Data)-1])
					default:
						if status_resp == len(addrs) || status_resp == -1 {
							status_resp = 0
							flag = true
						}
					}
					if flag {
						break
					}
				}
				fmt.Println("Nodes status:", n.Nodes)
				SendICMP(n, GetMsg())
			}
		}
		time.Sleep(60 * time.Second)
	}
}

func SendICMP(n *NearFieldDevice, data []byte) error {
	destIP := ""
	for key := range n.Nodes {
		if n.Nodes[key] == 1 {
			destIP = key
		}
	}
	if destIP != "" {
		n.TxQueue <- TxData{CreateIPData(n, "122.51.216.252", data), destIP, ADDR_LIST[destIP], RELAY_REQ}
		return nil
	} else {
		fmt.Println("All nodes are offline")
		return errors.New("all nodes are offline")
	}
}

func (n *NearFieldDevice) Init(s Strategy) error {
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

// func Run(ctx context.Context, configInfo chan ConfigInfo, devInfo chan DeviceInfo) error {
func (n *NearFieldDevice) Run(ctx context.Context) error {
	child_ctx, cancel = context.WithCancel(ctx)
	n.DevDesc.Device.Start(child_ctx)
	n.WG.Add(5)
	go Receiver(child_ctx, n)
	go PacketHandler(child_ctx, n)
	go Sender(child_ctx, n)
	go Timer(child_ctx, n)
	go NodesDetector(child_ctx, n)
	// go ConfigHandler(child_ctx, n, configInfo)
	n.running = true
	fmt.Println("nfc started")
	return nil
}

func (n *NearFieldDevice) Stop() error {
	n.DevDesc.Device.Stop()
	cancel()
	n.WG.Wait()
	fmt.Println("nfc stopped")
	n.running = false
	return nil
}

func (n *NearFieldDevice) Read(bufs *[][]byte, sizes *[]int, offset int) (int, error) {
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

func (n *NearFieldDevice) Write(bufs [][]byte, offset int) (int, error) {
	for i := offset; i < len(bufs); i++ {
		_, destIP, err := GetIP(bufs[i])
		if err != nil {
			fmt.Println(err)
			return i - offset, err
		}
		tx_type := P2P
		if destIP == BCST_IP {
			tx_type = BCST
		} else if _, ok := ADDR_LIST[destIP]; !ok && len(n.Nodes) > 0 {
			tx_type = RELAY_REQ
		}
		select {
		case n.TxQueue <- TxData{bufs[i], destIP, "", tx_type}:
		default:
			return i - offset, errors.New("tx queue is full")
		}
	}
	return len(bufs) - offset, nil
}

func (n *NearFieldDevice) BatchSize() (int, error) {
	return n.strategy.BatchSize, nil
}
