package nfd

import (
	"fmt"
	"gitee.com/czy_hit/log"

	"go.bug.st/serial/enumerator"

	"gitee.com/ccl0924/nfd/nfd/xbee"

	"context"
	"errors"
	"sync"
	"time"
)

const (
	BCST          = 0
	P2P           = 1
	ACK           = 2
	ADDR_REQ      = 3
	ADDR_RESP     = 4
	STATUS_REQ    = 5
	STATUS_RESP   = 6
	RELAY_REQ     = 7
	STATUS_UPDATE = 8

	XBEE      = 0
	BLUETOOTH = 1
	UNKNOWN   = 2

	STATUS_QUERY_REQ     = 0
	STATUS_QUERY_RESP    = 1
	SERACH_NODES_REQ     = 2
	SERACH_NODES_RESP    = 3
	STATUS_UPDATE_NOTIFY = 4

	RTT         = 5
	MAX_RETRIES = 3

	BCST_IP  = "255.255.255.255"
	BCST_MAC = "000000000000FFFF"

	DEFAULT_MAC  = "FFFFFFFFFFFFFFFF"
	DEFAULT_IPv4 = "192.168.0.1"
	DEFAULT_IPv6 = "1111:2222:3333:4444:5555:6666:7777:8888"
)

var (
	DevList = map[string]int{
		"2FE3:0100": BLUETOOTH,
		"0403:6001": XBEE,
	}

	AddrList = map[string]string{}

	logger log.Logger
)

type Strategy struct {
	BatchSize int
}

type ConfigInfo struct {
	Type   int
	Status int    // used by STATUS_QUERY_RESP and STATUS_UPDATE
	SrcMac string // used by STATUS_QUERY_RESP
}

type DeviceInfo struct {
	Type        int
	NodesStatus map[string]int // used by NODES_STATUS
	SrcMac      string         // used by STATUS_QUERY_REQ
}

type Device interface {
	SendPacket(data []byte, remoteAddr string) bool
	ReceivePacket() ([]byte, error)
	GetNodes() ([]string, error)
	Start(ctx context.Context)
	Stop()
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
	MAC     string
	Device  Device
}

type NearFieldDevice struct {
	DevDesc      DevDesc
	Seq          int
	IPv4         string
	IPv6         string
	NodeAddrs    []string
	NodeStatuses map[string]int
	AckQueue     chan Packet
	DataQueue    chan Packet
	StatusQueue  chan Packet
	RxData       chan []byte
	TimerPacket  TimerData
	Strategy     Strategy
	Running      bool
	Stopped      bool
	Started      map[string]bool

	Mutex  sync.Mutex
	Mutex2 sync.Mutex
	Mutex3 sync.Mutex
	WG     sync.WaitGroup

	before   int
	ackCount int
	context  context.Context
	cancel   context.CancelFunc
}

func NewNearFieldDevice(ipv4 string, ipv6 string) *NearFieldDevice {
	if ipv4 == "" {
		ipv4 = DEFAULT_IPv4
	}
	if ipv6 == "" {
		ipv6 = DEFAULT_IPv6
	}
	return &NearFieldDevice{
		IPv4:         ipv4,
		IPv6:         ipv6,
		Seq:          -1,
		NodeAddrs:    make([]string, 0),
		NodeStatuses: make(map[string]int),
		DataQueue:    make(chan Packet, 3),
		AckQueue:     make(chan Packet, 3),
		StatusQueue:  make(chan Packet, 32),
		RxData:       make(chan []byte, 64),
		Started:      make(map[string]bool),
		before:       -1,
	}
}

func DevIdf(n *NearFieldDevice) bool {
	portList, _ := enumerator.GetDetailedPortsList()
	for _, port := range portList {
		id := fmt.Sprintf("%s:%s", port.VID, port.PID)
		if devType, ok := DevList[id]; ok {
			n.DevDesc = DevDesc{devType, port.Name, DEFAULT_MAC, nil}
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
			n.DevDesc.MAC = xbee.MAC
			n.DevDesc.Device = xbee
		case BLUETOOTH:
			// Bluetooth device initialization
		}
	} else {
		return errors.New("failed to identify the device type")
	}
	return nil
}

func (n *NearFieldDevice) Send(tx TxData) bool {
	n.Mutex3.Lock()
	defer n.Mutex3.Unlock()

	n.Seq = (n.Seq + 1) % 256
	srcMac := n.DevDesc.MAC

	if tx.TxType == BCST {
		p := NewPacket(n.Seq, BCST, srcMac, BCST_MAC, tx.Data)
		data, err := p.Encode()
		if err != nil {
			return false
		}
		if n.DevDesc.Device.SendPacket(data, p.DestMac) {
			logger.Infof("send BCST %v", p.String())
			return true
		} else {
			logger.Warnf("send BCST packet failed")
			return false
		}
	} else if tx.TxType == STATUS_REQ {
		p := NewPacket(n.Seq, STATUS_REQ, srcMac, tx.DestMac, tx.Data)
		data, err := p.Encode()
		if err != nil {
			return false
		}
		if n.DevDesc.Device.SendPacket(data, p.DestMac) {
			logger.Infof("send STATUS_REQ %v", p.String())
			n.TimerPacket = TimerData{*p, RTT, 0}
			if n.WaitForAck(p.Seq) {
				return true
			} else {
				logger.Warnf("a STATUS_REQ was sent, but no STATUS_RESP was received")
				return false
			}
		} else {
			logger.Warnf("send STATUS_REQ packet failed")
			return false
		}
	} else if tx.TxType == STATUS_RESP {
		p := NewPacket(n.Seq, STATUS_RESP, n.DevDesc.MAC, tx.DestMac, tx.Data)
		data, err := p.Encode()
		if err != nil {
			return false
		}
		if n.DevDesc.Device.SendPacket(data, p.DestMac) {
			logger.Infof("send STATUS_RESP %v", p)
			return true
		} else {
			logger.Warnf("send STATUS_RESP packet failed")
			return false
		}
	} else if tx.TxType == RELAY_REQ {
		p := NewPacket(n.Seq, RELAY_REQ, srcMac, tx.DestMac, tx.Data)
		for key := range n.NodeStatuses {
			if n.NodeStatuses[key] == 1 {
				p.DestMac = AddrList[key]
				data, err := p.Encode()
				if err != nil {
					return false
				}
				if n.DevDesc.Device.SendPacket(data, p.DestMac) {
					logger.Infof("send RELAY_REQ %v", p.String())
					return true
				} else {
					continue
				}
			}
		}
		logger.Warnf("send RELAY_REQ packet failed")
		return false
	} else {
		if destMac, ok := AddrList[tx.DestIP]; ok {
			p := NewPacket(n.Seq, P2P, srcMac, destMac, tx.Data)
			data, err := p.Encode()
			if err != nil {
				return false
			}
			if n.DevDesc.Device.SendPacket(data, p.DestMac) {
				logger.Infof("send P2P %v", p.String())
				n.TimerPacket = TimerData{*p, RTT, MAX_RETRIES}
				if n.WaitForAck(p.Seq) {
					return true
				} else {
					logger.Warnf("a P2P was sent, but no ACK was received")
					return false
				}
			} else {
				logger.Warnf("send P2P packet failed")
				return false
			}
		} else {
			p := NewPacket(n.Seq, ADDR_REQ, srcMac, BCST_MAC, CreateIPData(n, tx.DestIP, nil))
			data, err := p.Encode()
			if err != nil {
				return false
			}
			if n.DevDesc.Device.SendPacket(data, p.DestMac) {
				logger.Infof("send ADDR_REQ %v", p.String())
				n.TimerPacket = TimerData{*p, RTT, MAX_RETRIES - 2}
				if n.WaitForAck(p.Seq) {
					n.Seq = (n.Seq + 1) % 256
					destMac = AddrList[tx.DestIP]
					p := NewPacket(n.Seq, P2P, srcMac, destMac, tx.Data)
					data, err := p.Encode()
					if err != nil {
						return false
					}
					if n.DevDesc.Device.SendPacket(data, p.DestMac) {
						logger.Infof("send P2P %v", p.String())
						n.TimerPacket = TimerData{*p, RTT, MAX_RETRIES}
						if n.WaitForAck(p.Seq) {
							return true
						} else {
							logger.Warnf("a P2P was sent, but no ACK was received")
							return false
						}
					} else {
						logger.Warnf("send P2P packet failed")
						return false
					}
				} else {
					logger.Warnf("a ADDR_REQ was sent, but no ADDR_RESP was received")
					return false
				}
			} else {
				logger.Warnf("send ADDR_REQ packet failed")
				return false
			}
		}
	}
}

func (n *NearFieldDevice) WaitForAck(seq int) bool {
	count := 0
	for {
		select {
		case ack := <-n.AckQueue:
			if ack.Seq == seq {
				if ack.PacketType == ACK {
					if ack.Seq != n.before {
						logger.Infof("received a ACK for [%v]", seq)
						n.ackCount++
						logger.Infof("count: %d", n.ackCount)
						n.before = ack.Seq
					}
				} else if ack.PacketType == ADDR_RESP {
					srcIP, _, err := GetIP(ack.Data)
					if err == nil {
						AddrList[srcIP] = ack.SrcMac
					}
					logger.Infof("received a ADDR_RESP for [%d]", seq)
				} else if ack.PacketType == STATUS_RESP {
					logger.Infof("received a STATUS_RESP for [%d]", seq)
				}
				n.TimerPacket.Timeout = 2147483647
				return true
			}
		default:
			if n.TimerPacket.Retries == 0 {
				for count < RTT {
					count++
					time.Sleep(1 * time.Second)
					if len(n.AckQueue) > 0 {
						break
					}
				}
				if count == RTT {
					logger.Warnf("stop waiting for ack")
					return false
				}
			}
		}
	}
}

func (n *NearFieldDevice) Receiver(ctx context.Context) {
	defer n.WG.Done()
	n.Mutex2.Lock()
	if !n.Started["Receiver"] {
		n.Started["Receiver"] = true
		logger.Infof("Receiver Started")
	} else {
		return
	}
	n.Mutex2.Unlock()
	for {
		select {
		case <-ctx.Done():
			logger.Infof("Receiver Stopped")
			n.Mutex2.Lock()
			n.Started["Receiver"] = false
			n.Mutex2.Unlock()
			return
		default:
			data, err := n.DevDesc.Device.ReceivePacket()
			if err == nil {
				p, err := DecodePacket(data)
				if err == nil {
					if p.PacketType == ACK || p.PacketType == ADDR_RESP {
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

func (n *NearFieldDevice) PacketHandler(ctx context.Context, deviceCh chan DeviceInfo) {
	defer n.WG.Done()
	n.Mutex2.Lock()
	if !n.Started["PacketHandler"] {
		n.Started["PacketHandler"] = true
		logger.Infof("PacketHandler Started")
	} else {
		return
	}
	n.Mutex2.Unlock()
	for {
		select {
		case p := <-n.DataQueue:
			SrcIP, DestIP, err := GetIP(p.Data)
			if err == nil {
				AddrList[SrcIP] = p.SrcMac
				if p.PacketType == ADDR_REQ {
					if n.IPv4 == DestIP || n.IPv6 == DestIP {
						p := NewPacket(p.Seq, ADDR_RESP, n.DevDesc.MAC, p.SrcMac, CreateIPData(n, SrcIP, nil))
						data, err := p.Encode()
						if err == nil {
							if n.DevDesc.Device.SendPacket(data, p.DestMac) {
								logger.Infof("received a ADDR_REQ packet, send ADDR_RESP %v", p)
							} else {
								logger.Warnf("send ADDR_RESP packet failed")
							}
						}
					}
				} else if p.PacketType == STATUS_REQ {
					// If a STATUS_REQ is received from another node, send the STATUS_QUERY_REQ to the upper layer
					deviceCh <- DeviceInfo{STATUS_QUERY_REQ, nil, p.SrcMac}
				} else if p.PacketType == STATUS_UPDATE {
					n.NodeStatuses[SrcIP] = int(p.Data[len(p.Data)-1])
				} else if p.PacketType == RELAY_REQ {
					// If a RELAY_REQ is received, it is added to the RxData, and the upper layer reads it and relay it
					n.RxData <- p.Data
				} else {
					n.RxData <- p.Data
					if p.PacketType == P2P {
						p := NewPacket(p.Seq, ACK, n.DevDesc.MAC, p.SrcMac, CreateIPData(n, SrcIP, nil))
						data, err := p.Encode()
						if err == nil {
							n.DevDesc.Device.SendPacket(data, p.DestMac)
						}
					} else {
						logger.Infof("received a BCST %v", p.String())
					}
				}
			} else {
				logger.Infof("get ip address failed")
			}
		case <-ctx.Done():
			logger.Infof("PacketHandler Stopped")
			n.Mutex2.Lock()
			n.Started["PacketHandler"] = false
			n.Mutex2.Unlock()
			return
		}
	}
}

func (n *NearFieldDevice) Timer(ctx context.Context) {
	defer n.WG.Done()
	n.Mutex2.Lock()
	if !n.Started["Timer"] {
		n.Started["Timer"] = true
		logger.Infof("Timer Started")
	} else {
		return
	}
	n.Mutex2.Unlock()
	for {
		select {
		case <-ctx.Done():
			n.Mutex2.Lock()
			n.Started["Timer"] = false
			n.Mutex2.Unlock()
			logger.Infof("Timer Stopped")
			return
		default:
			n.TimerPacket.Timeout--
			if n.TimerPacket.Timeout == 0 {
				if n.TimerPacket.Retries > 0 {
					p := n.TimerPacket.Packet
					data, err := p.Encode()
					if err == nil {
						if n.DevDesc.Device.SendPacket(data, p.DestMac) {
							logger.Infof("send RT %v", p.String())
							n.TimerPacket.Timeout = RTT
						} else {
							logger.Warnf("send RT packet failed")
						}
					}
					n.TimerPacket.Retries--
				}
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func (n *NearFieldDevice) NodeDetector(ctx context.Context) {
	defer n.WG.Done()
	n.Mutex2.Lock()
	if !n.Started["NodeDetector"] {
		n.Started["NodeDetector"] = true
		logger.Infof("NodeDetector Started")
	} else {
		return
	}
	n.Mutex2.Unlock()
	for {
		select {
		case <-ctx.Done():
			n.Mutex2.Lock()
			n.Started["NodeDetector"] = false
			n.Mutex2.Unlock()
			logger.Infof("NodeDetector Stopped")
			return
		default:
			logger.Infof("Start discovering nodes...")
			addrs, err := n.DevDesc.Device.GetNodes()
			logger.Infof("Found %d nodes", len(addrs))
			if err == nil {
				n.NodeAddrs = addrs
			}
		}
	}
}

func (n *NearFieldDevice) UpdateStatus() {
	if len(n.NodeAddrs) == 0 {
		return
	}
	for i := range n.NodeAddrs {
		tx := TxData{CreateIPData(n, DEFAULT_IPv4, nil), DEFAULT_IPv4, n.NodeAddrs[i], STATUS_REQ}
		n.Send(tx)
	}
	keys := make(map[string]int)
	select {
	case resp := <-n.StatusQueue:
		n.Mutex.Lock()
		SrcIP, _, err := GetIP(resp.Data)
		if err == nil {
			AddrList[SrcIP] = resp.SrcMac
		}
		n.NodeStatuses[SrcIP] = int(resp.Data[len(resp.Data)-1])
		n.Mutex.Unlock()
		keys[SrcIP] = 1
	default:
		n.Mutex.Lock()
		for key := range AddrList {
			if _, ok := keys[key]; !ok {
				delete(AddrList, key)
			}
		}
		for key := range n.NodeStatuses {
			if _, ok := keys[key]; !ok {
				delete(n.NodeStatuses, key)
			}
		}
		n.Mutex.Unlock()
		logger.Infof("Nodes status %v", n.NodeStatuses)
		return
	}
}

func (n *NearFieldDevice) ConfigHandler(ctx context.Context, configCh chan ConfigInfo, deviceCh chan DeviceInfo) {
	defer n.WG.Done()
	n.Mutex2.Lock()
	if !n.Started["ConfigHandler"] {
		n.Started["ConfigHandler"] = true
		logger.Infof("ConfigHandler Started")
	} else {
		return
	}
	n.Mutex2.Unlock()
	for {
		select {
		case <-ctx.Done():
			n.Mutex2.Lock()
			n.Started["ConfigHandler"] = false
			n.Mutex2.Unlock()
			logger.Infof("ConfigHandler Stopped")
			return
		case config := <-configCh:
			if config.Type == SERACH_NODES_REQ {
				// If a SERACH_NODES_REQ is received from the upper layer,
				// the node in the current network is returned to it
				n.UpdateStatus()
				deviceCh <- DeviceInfo{SERACH_NODES_RESP, n.NodeStatuses, ""}
			} else if config.Type == STATUS_QUERY_RESP {
				// If a STATUS_QUERY_RESP is received from the upper layer,
				// a STATUS_RESP is sent to the requesting node
				tx := TxData{CreateIPData(n, DEFAULT_IPv4, []byte{byte(config.Status)}), "", config.SrcMac, STATUS_RESP}
				n.Send(tx)
			} else if config.Type == STATUS_UPDATE_NOTIFY {
				tx := TxData{CreateIPData(n, BCST_IP, []byte{byte(config.Status)}), "", BCST_MAC, STATUS_UPDATE}
				n.Send(tx)
			}
		default:
			// DO NOTHING
		}
	}
}

func (n *NearFieldDevice) Init(s Strategy) error {
	logger = log.GetLogger()
	logger.Infof("Initializing...")
	n.Strategy = s
	err := n.Open()
	if err != nil {
		logger.Warnf("Initialize failed")
		return err
	}
	logger.Infof("Initialize finished")
	return nil
}

func (n *NearFieldDevice) Run(ctx context.Context, configInfo chan ConfigInfo, devInfo chan DeviceInfo) error {
	if n.DevDesc.Device == nil {
		logger.Warnf("Please initialize device first")
		return errors.New("run failed")
	}
	n.context, n.cancel = context.WithCancel(ctx)
	n.DevDesc.Device.Start(n.context)
	n.WG.Add(4)
	go n.Receiver(n.context)
	go n.PacketHandler(n.context, devInfo)
	go n.Timer(n.context)
	go n.ConfigHandler(n.context, configInfo, devInfo)
	// go NodeDetector(child_ctx)
	return nil
}

func (n *NearFieldDevice) Stop() error {
	if n.DevDesc.Device == nil {
		logger.Warnf("Please initialize device first")
		return errors.New("stop failed")
	}
	n.DevDesc.Device.Stop()
	n.cancel()
	n.WG.Wait()
	return nil
}

func (n *NearFieldDevice) Read(buf []byte) (size int, err error) {
	data := <-n.RxData
	size = copy(buf, data)
	return size, err
}

func (n *NearFieldDevice) Write(buf []byte) (int, error) {
	data := make([]byte, len(buf))
	size := copy(data, buf)
	_, destIP, err := GetIP(data)
	if err != nil {
		return 0, err
	}
	tx_type := P2P
	if destIP == BCST_IP {
		tx_type = BCST
	} else if _, ok := AddrList[destIP]; len(n.NodeStatuses) > 0 && !ok {
		// If the ADDR_LIST contains all nodes in the current network,
		// but the destination IP address of the packet is not in it,
		// the packet is considered to be of the RELAY_REQ type,
		// which needs to be relayed to server by other nodes
		tx_type = RELAY_REQ
	}
	tx := TxData{data, destIP, "", tx_type}
	if !n.Send(tx) {
		return 0, errors.New("send packet failed")
	}

	return size, nil
}

func (n *NearFieldDevice) BatchSize() (int, error) {
	return n.Strategy.BatchSize, nil
}
