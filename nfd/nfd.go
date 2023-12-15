package nfd

import (
	"fmt"

	"gitee.com/czy_hit/log"
	"go.bug.st/serial/enumerator"

	// Test for virtual device
	// If you need to use a physical device, replace it with "ccl/go/nfd/xbee"
	// xbee "gitee.com/ccl0924/nfd/nfd/virtual_xbee"

	"gitee.com/ccl0924/nfd/nfd/wifi"
	"gitee.com/ccl0924/nfd/nfd/xbee"

	"context"
	"errors"
	"strings"
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

	UNKNOWN   = 0
	BLUETOOTH = 1
	WIFI      = 2
	XBEE      = 3

	STATUS_QUERY_REQ     = 0
	STATUS_QUERY_RESP    = 1
	SERACH_NODES_REQ     = 2
	SERACH_NODES_RESP    = 3
	STATUS_UPDATE_NOTIFY = 4

	TIMEOUT = 5

	BCST_IPv4    = "255.255.255.255"
	DEFAULT_IPv4 = "192.168.101.128"
	DEFAULT_IPv6 = "fe80::1"

	VIRTUAL_IP_PREFIX = "192.168.101."
)

var (
	logger log.Logger

	DEV_LIST = map[string]int{
		"2FE3:0100":          BLUETOOTH,
		"0403:6001:A50285BI": XBEE,
		//"0403:6001": XBEE,
		"1A86:7523": WIFI,
	}

	ADDR_LIST = map[string]string{}

	RTT_MAP = map[string]time.Duration{}

	//RELAY_TABLE = []string{}

	BCST_MAC = ""
	P2P_MAC  = ""
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
	NodesStatus map[string]int // used by SERACH_NODES_RESP
	SrcMac      string         // used by STATUS_QUERY_REQ
}

type Device interface {
	SendPacket(data []byte, remoteAddr string) bool
	ReceivePacket() ([]byte, error)
	GetNodes() ([]string, error)
	Start(ctx context.Context)
	Stop()
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
	Strategy     Strategy
	Started      map[string]bool
	ServerAddr   string

	Mutex  sync.Mutex
	Mutex2 sync.Mutex
	Mutex3 sync.Mutex
	Mutex4 sync.Mutex
	Mutex5 sync.Mutex
	WG     sync.WaitGroup

	before    int
	ack_count int
	context   context.Context
	cancel    context.CancelFunc

	// Test for virtual device
	// AddrMap    map[string]string
	// UDPPort    int
	// MACAddress string
}

func NewNearFieldDevice(ipv4 string, ipv6 string) *NearFieldDevice {
	// func NewNearFieldDevice(ipv4 string, ipv6 string, MACAddress string, AddrMap map[string]string, UDPPort int) *NearFieldDevice {
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
		DataQueue:    make(chan Packet, 10),
		AckQueue:     make(chan Packet, 10),
		StatusQueue:  make(chan Packet, 10),
		RxData:       make(chan []byte, 128),
		Started:      make(map[string]bool),
		before:       -1,

		// Test for virtual device
		// AddrMap:    AddrMap,
		// UDPPort:    UDPPort,
		// MACAddress: MACAddress,
	}
}

func DevIdf(n *NearFieldDevice) bool {
	portList, _ := enumerator.GetDetailedPortsList()
	for _, port := range portList {
		id := fmt.Sprintf("%s:%s:%s", port.VID, port.PID, port.SerialNumber)
		//id := fmt.Sprintf("%s:%s", port.VID, port.PID)
		if devType, ok := DEV_LIST[id]; ok {
			n.DevDesc = DevDesc{devType, port.Name, P2P_MAC, nil}
			return true
		}
	}
	return false
}

func (n *NearFieldDevice) Open() error {
	if DevIdf(n) {
		switch n.DevDesc.DevType {
		case XBEE:
			xb, err := xbee.NewXbee(n.DevDesc.DevPort, 230400)
			//xb, err := xbee.NewXbee("COM7", 230400)

			if err != nil {
				return err
			}
			n.DevDesc.MAC = xb.MAC
			n.DevDesc.Device = xb
			BCST_MAC = "000000000000FFFF"
			P2P_MAC = "FFFFFFFFFFFFFFFF"
		case BLUETOOTH:
			// Bluetooth device initialization
		case WIFI:
			wf, err := wifi.NewWifi(n.DevDesc.DevPort, 921600)

			// wifi, err := wifi.NewWifi("COM3", 921600)
			if err != nil {
				return err
			}
			n.DevDesc.MAC = wf.MAC
			n.DevDesc.Device = wf
			BCST_MAC = "1111111111"
			P2P_MAC = "1111111111"
		}
	} else {
		return errors.New("failed to identify the device type")
	}
	return nil
}

// Test for virtual device
// If you need to use a physical device, replace it with the code commented out above
// func (n *NearFieldDevice) Open() error {
// 	xbee, err := xbee.NewXbee(n.DevDesc.DevPort, 115200, n.MACAddress, n.AddrMap, n.UDPPort)
// 	if err != nil {
// 		return err
// 	}
// 	n.DevDesc.MAC = xbee.MAC
// 	n.DevDesc.Device = xbee
// 	return nil
// }

func (n *NearFieldDevice) Tx(tx TxData, nextSeq int) bool {
	srcMac := n.DevDesc.MAC

	seq := 0
	if nextSeq != -1 {
		seq = nextSeq
	} else {
		n.Seq = (n.Seq + 1) % 256
		seq = n.Seq
	}

	p := NewPacket(seq, tx.TxType, srcMac, tx.DestMac, tx.Data)
	data, err := p.Encode()
	if err != nil {
		logger.Debugf("encode packet error: %v", err)
		return false
	}

	var txType string
	needACK := false

	switch tx.TxType {
	case BCST:
		txType = "BCST"
	case P2P:
		txType = "P2P"
		needACK = true
	case ADDR_REQ:
		txType = "ADDR_REQ"
		needACK = true
	case ADDR_RESP:
		txType = "ADDR_RESP"
	case STATUS_REQ:
		txType = "STATUS_REQ"
		needACK = true
	case STATUS_RESP:
		txType = "STATUS_RESP"
	case RELAY_REQ:
		txType = "RELAY_REQ"
		needACK = true
	case ACK:
		txType = "ACK"
	default:
		txType = "UNKNOWN"
	}

	if n.DevDesc.Device.SendPacket(data, p.DestMac) {
		logger.Debugf("send "+txType+" %v", p.String())
		if needACK {
			current := time.Now()
			if n.WaitForAck(p.Seq) {
				n.Mutex5.Lock()
				RTT_MAP[n.IPv4] = time.Since(current)
				n.Mutex5.Unlock()
				return true
			} else {
				logger.Debugf("a " + txType + " sent, but no ACK received")
				return false
			}
		}
		return true
	} else {
		logger.Debugf("send " + txType + " packet failed")
		return false
	}
}

func (n *NearFieldDevice) Send(tx TxData, nextSeq int) bool {
	n.Mutex3.Lock()
	defer n.Mutex3.Unlock()

	if !strings.HasPrefix(tx.DestIP, VIRTUAL_IP_PREFIX) {
		return false
	}

	if tx.TxType == P2P {
		if destMac, ok := ADDR_LIST[tx.DestIP]; ok {
			tx.DestMac = destMac
			return n.Tx(tx, nextSeq)
		} else {
			if tx.DestIP == n.ServerAddr {
				tx.TxType = RELAY_REQ
				tx.DestMac = n.NodeAddrs[0]
				return n.Tx(tx, nextSeq)
			}
			addrReq := TxData{CreateIPData(n, tx.DestIP, nil), tx.DestIP, BCST_MAC, ADDR_REQ}
			if n.Tx(addrReq, nextSeq) {
				tx.DestMac = ADDR_LIST[addrReq.DestIP]
				return n.Tx(tx, nextSeq)
			} else if len(n.NodeAddrs) > 0 {
				tx.DestMac = n.NodeAddrs[0]
				tx.TxType = RELAY_REQ
				if n.Tx(tx, nextSeq) {
					n.ServerAddr = tx.DestIP
					return true
				}
				logger.Debugf("a ADDR_REQ sent, but no ADDR_RESP received")
				return false
			}
			return false
		}
	} else {
		return n.Tx(tx, nextSeq)
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
						logger.Debugf("received a ACK for [%v]", seq)
						n.ack_count++
						//logger.Infof("count: %d", n.ack_count)
						n.before = ack.Seq
					}
				} else if ack.PacketType == ADDR_RESP {
					srcIP, _, err := GetIP(ack.Data)
					if err == nil {
						ADDR_LIST[srcIP] = ack.SrcMac
					}
					logger.Debugf("received a ADDR_RESP for [%d]", seq)
				} else if ack.PacketType == STATUS_RESP {
					logger.Debugf("received a STATUS_RESP for [%d]", seq)
				}
				return true
			}
		default:
			for count < TIMEOUT*10 {
				count++
				time.Sleep(100 * time.Millisecond)
				if len(n.AckQueue) > 0 {
					break
				}
			}
			if count == TIMEOUT*10 {
				logger.Debugf("stop waiting for ack")
				return false
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
			if err != nil {
				continue
			}

			p, err := DecodePacket(data)
			if err != nil {
				logger.Debugf("Error decoding packet: %v", err)
				continue
			}

			switch p.PacketType {
			case ACK, ADDR_RESP:
				n.AckQueue <- *p
			case STATUS_RESP:
				n.AckQueue <- *p
				n.StatusQueue <- *p
			default:
				n.DataQueue <- *p
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
			if err != nil {
				logger.Debugf("get ip address failed")
			} else {
				switch p.PacketType {
				case ADDR_REQ:
					if n.IPv4 == DestIP || n.IPv6 == DestIP {
						addrResp := TxData{CreateIPData(n, SrcIP, nil), SrcIP, p.SrcMac, ADDR_RESP}
						n.Send(addrResp, p.Seq)
					}
				case STATUS_REQ:
					// If a STATUS_REQ is received from another node, send the STATUS_QUERY_REQ to the upper layer.
					deviceCh <- DeviceInfo{STATUS_QUERY_REQ, nil, p.SrcMac}
				case STATUS_UPDATE:
					n.NodeStatuses[SrcIP] = int(p.Data[len(p.Data)-1])
				default:
					// If a P2P or RELAY_REQ received, it is added to the RxData, and the upper layer reads it or relay it.
					n.RxData <- p.Data

					if p.PacketType == P2P || p.PacketType == RELAY_REQ {
						p := NewPacket(p.Seq, ACK, n.DevDesc.MAC, p.SrcMac, CreateIPData(n, SrcIP, nil))
						data, err := p.Encode()
						if err != nil {
							logger.Debugf("encode packet error: %v", err)
							continue
						}
						if n.DevDesc.Device.SendPacket(data, p.DestMac) {
							logger.Debugf("send ACK %v", p.String())
						}
					} else {
						logger.Debugf("received a BCST %v", p.String())
					}
				}
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
			// logger.Infof("Start discovering nodes...")
			addrs, err := n.DevDesc.Device.GetNodes()
			if err == nil {
				n.Mutex4.Lock()
				if len(addrs) != 0 {
					logger.Debugf("found %d nodes: %v", len(addrs), addrs)
					n.NodeAddrs = addrs
				}
				n.Mutex4.Unlock()
			} else {
				logger.Debugf("get nodes error: %v", err)
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
		n.Send(tx, -1)
	}
	keys := make(map[string]int)
	select {
	case resp := <-n.StatusQueue:
		n.Mutex.Lock()
		SrcIP, _, err := GetIP(resp.Data)
		if err == nil {
			ADDR_LIST[SrcIP] = resp.SrcMac
		}
		n.NodeStatuses[SrcIP] = int(resp.Data[len(resp.Data)-1])
		n.Mutex.Unlock()
		keys[SrcIP] = 1
	default:
		n.Mutex.Lock()
		for key := range ADDR_LIST {
			if _, ok := keys[key]; !ok {
				delete(ADDR_LIST, key)
			}
		}
		for key := range n.NodeStatuses {
			if _, ok := keys[key]; !ok {
				delete(n.NodeStatuses, key)
			}
		}
		n.Mutex.Unlock()
		logger.Debugf("Nodes status %v", n.NodeStatuses)
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
				n.Send(tx, -1)
			} else if config.Type == STATUS_UPDATE_NOTIFY {
				tx := TxData{CreateIPData(n, BCST_IPv4, []byte{byte(config.Status)}), "", BCST_MAC, STATUS_UPDATE}
				n.Send(tx, -1)
			}
		default:
			// DO NOTHING
		}
	}
}

func (n *NearFieldDevice) GetLatestRTT() map[string]time.Duration {
	res := make(map[string]time.Duration)
	n.Mutex5.Lock()
	for key, value := range RTT_MAP {
		res[key] = value
		RTT_MAP[key] = time.Duration(0)
	}
	n.Mutex5.Unlock()
	return res
}

func (n *NearFieldDevice) GetMacAddress() string {
	if mac := n.DevDesc.MAC; mac != P2P_MAC {
		return mac
	} else {
		return "UNKNOWN"
	}
}

func (n *NearFieldDevice) BatchSize() (int, error) {
	return n.Strategy.BatchSize, nil
}

func (n *NearFieldDevice) Init(s Strategy) error {
	logger = log.GetLogger()
	logger.Infof("Initializing...")
	n.Strategy = s
	err := n.Open()
	if err != nil {
		logger.Warnf(err.Error())
		return err
	}
	RTT_MAP[n.IPv4] = time.Duration(0)
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
	n.WG.Add(3)
	go n.Receiver(n.context)
	go n.PacketHandler(n.context, devInfo)
	go n.NodeDetector(n.context)
	//go n.ConfigHandler(n.context, configInfo, devInfo)
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
	txType := P2P
	if destIP == BCST_IPv4 {
		txType = BCST
	}
	tx := TxData{data, destIP, "", txType}
	if !n.Send(tx, -1) {
		return 0, errors.New("send packet failed")
	}

	return size, nil
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
