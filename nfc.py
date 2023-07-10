import xbee
import threading
from serial.tools import list_ports
import logging
import struct
import threading
import logging
import copy
import queue
import time

# newest code here

class Packet:
    def __init__(self, seq: int, packet_type: int, src_mac: str, \
                 dest_mac: str, src_ip: str, dest_ip: str, data) -> None:
        self.seq = seq
        self.packet_type = packet_type
        self.src_mac = src_mac
        self.dest_mac = dest_mac
        self.src_ip = src_ip
        self.dest_ip = dest_ip
        self.data = data


    def encode(self) -> bytes:
        src_ip = ip2hex(self.src_ip).encode('UTF-8')
        dest_ip = ip2hex(self.dest_ip).encode('UTF-8')
        src_mac = self.src_mac.encode('UTF-8')
        dest_mac = self.dest_mac.encode('UTF-8')
        data = self.data.encode('UTF-8')

        packet_data = struct.pack(f'>BB16s16s8s8s', self.seq, self.packet_type, \
                                  src_mac, dest_mac, src_ip, dest_ip) + data

        return packet_data
    

    @classmethod
    def decode(cls, data: bytes) -> 'Packet':
        seq, packet_type, src_mac, dest_mac, src_ip, \
            dest_ip = struct.unpack('>BB16s16s8s8s', data[:50])
        data = data[50:].decode('UTF-8')

        return cls(seq, packet_type, src_mac.decode('UTF-8'), dest_mac.decode('UTF-8'), \
                   hex2ip(src_ip.decode('UTF-8')), hex2ip(dest_ip.decode('UTF-8')), data)
    

    def __repr__(self) -> str:
        return f"Packet(seq={self.seq}, packet_type={self.packet_type}, src_mac={self.src_mac}, dest_mac={self.dest_mac}, src_ip={self.src_ip}, dest_ip={self.dest_ip}, data={self.data})"
    
    

def ip2hex(ip: str) -> str:
    list = ip.split('.')
    hex_list = [format(int(item), '02x') for item in list]
    return ''.join(hex_list)


def hex2ip(hex: str) -> str:
    ip_list = []
    while len(hex) > 0:
        ip_list.append(str(int(hex[:2], 16)))
        ip_list.append('.')
        hex = hex[2:]
    return ''.join(ip_list[:-1])
    


class NFC:

    BCST = 0
    ADDR_REQ = 1
    ADDR_RESP = 2
    P2P = 3
    RT_REQ = 4
    RT_RESP = 5
    ACK = 6

    BCST_ADDR  = 'FFFFFFFFFFFFFFFF'

    DEV_LIST = {
        '1027:24577': 'Xbee'
    }

    ADDR_LIST = {}

    def __init__(self, ip: str, transmission_timeout=30):
        self.port_list = []
        self.dev_type = None
        self.dev_port = None
        self.device = None
        self.mac = 0
        self.seq = -1
        self.recv_seq = {}
        self.ip = ip
        self.pending_data = {}
        self.tx_map = {}
        self.rx_map = {}
        self.rx_queue = queue.Queue()
        self.rt_queue = queue.Queue()
        self.sending = False
        self.started = False
        self.timer_map = {}
        self.timeout = transmission_timeout

        self.mutex = threading.Lock()
        self.mutex1 = threading.Lock()
        self.mutex2 = threading.Lock()
        self.mutex3 = threading.Lock()


    def dev_idf(self) -> bool:
        try:
            self.port_list = list(list_ports.comports())
            for port in self.port_list:
                id = f"{port.vid}:{port.pid}"
                dev_type = self.DEV_LIST.get(id)
                if dev_type is not None:
                    self.dev_type = dev_type
                    self.dev_port = port.device
                    return True
        except Exception as e:
            print(f"Error occurred when identifying the device type: {e}")

        return False
    
    
    def open(self, baudrate=230400) -> bool:
        try:
            if self.dev_idf():
                if self.dev_type == 'Xbee':
                    self.device = xbee.Xbee(self.dev_port, baudrate)
                    self.mac = self.device.dev.address.hex()
                elif self.dev_type == 'Bluetooth':
                    pass
                return True
        except Exception as e:
            print(f"Error occurred when opening the serial port: {e}")
        
        return False
            

    def send(self, data: bytes, dest_ip='0.0.0.0') -> bool:
        try:
            data = data.decode('UTF-8')
            res = False
            self.seq = (self.seq + 1) % 256
            if dest_ip == '0.0.0.0':
                p = Packet(self.seq, self.BCST, self.mac, \
                           self.BCST_ADDR, self.ip, dest_ip, data)
                with self.mutex:
                    self.tx_map[self.seq] = p
                res = self.device.send_packet(p.encode())
                print(f"INFO: send BCST {p}")
            else:
                with self.mutex1:
                    dest_mac = self.ADDR_LIST.get(dest_ip)
                if dest_mac is not None:
                    p = Packet(self.seq, self.P2P, self.mac, \
                                dest_mac, self.ip, dest_ip, data)
                    with self.mutex:
                        self.tx_map[self.seq] = p
                    res = self.device.send_packet(p.encode(), remoteAddr=dest_mac)
                    print(f"INFO: send P2P {p}")
                    with self.mutex3:
                        self.update_timer_map(self.P2P, 0, packet=p, op=0)
                else:
                    p = Packet(self.seq, self.ADDR_REQ, self.mac, \
                               self.BCST_ADDR, self.ip, dest_ip, 'None')
                    if self.pending_data.get(dest_ip) is None:   
                        with self.mutex1:
                            self.pending_data[dest_ip] = [[p.seq, data]]
                            res = self.device.send_packet(p.encode())
                            print(f"INFO: send ADDR_REQ {p}")
                        with self.mutex3:
                            self.update_timer_map(self.ADDR_REQ, 0, packet=p, op=0)
                    else:
                        with self.mutex1:
                            self.pending_data[dest_ip].append([p.seq, data])
                        res = True
            
            # test code
            self.seq = (self.seq + 1) % 256
            # test code end

            with self.mutex:
                if len(self.tx_map) > 60:
                    keys_to_remove = list(self.tx_map.keys())[:30]
                    for key_to_remove in keys_to_remove:
                        del self.tx_map[key_to_remove]

        except Exception as e:
            print(f"Error occurred when sending data: {e}")
            res = False
        
        return res
    
    
    def send_pending_data(self, dest_ip: str):
        try:
            with self.mutex1:
                packets = self.pending_data.get(dest_ip)
                dest_mac = self.ADDR_LIST.get(dest_ip)
                del self.pending_data[dest_ip]
            if packets is not None:
                for p in packets:
                    p = Packet(p[0], self.P2P, self.mac, \
                               dest_mac, self.ip, dest_ip, p[1])
                    with self.mutex:
                        self.tx_map[p.seq] = p
                    self.device.send_packet(p.encode(), remoteAddr=dest_mac)
                    print(f"INFO: send pending data {p}")
                    with self.mutex3:
                        self.update_timer_map(self.P2P, 0, packet=p, op=0)
        except Exception as e:
            print(f"Error occurred when sending pending data: {e}")
    
    
    def recv(self):
        while self.started:
            data = self.device.receive_packet()
            p = Packet.decode(data)
            self.rx_queue.put(p)


    def handler(self):
        while self.started:
            if self.rx_queue.empty():
                continue
            p = self.rx_queue.get()

            if self.recv_seq.get(p.src_mac) is None: 
                self.recv_seq[p.src_mac] = -1

            if p.packet_type == self.RT_REQ:
                with self.mutex:
                    # test code
                    self.tx_map[1] = '6666666666'
                    # test code end
                    p = Packet(p.seq, self.RT_RESP, p.dest_mac, p.src_mac, \
                               p.dest_ip, p.src_ip, self.tx_map.get(p.seq))
                self.device.send_packet(p.encode(), remoteAddr=p.dest_mac)
                print(f"INFO: received a RT_REQ packet for seq:[{p.seq}], send RT_RESP {p}")
                with self.mutex3:
                    for key in self.tx_map:
                        if key > p.seq:
                            self.update_timer_map(self.P2P, seq=key, packet=None, op=1)
            elif p.packet_type == self.RT_RESP:
                print(f"INFO: received a RT_RESP {p}")
                with self.mutex3:
                    self.update_timer_map(self.RT_REQ, seq=p.seq, packet=None, op=1)
                with self.mutex2:
                    if self.rx_map.get(p.src_ip) is None:
                        self.rx_map[p.src_ip] = [p.data]
                    else:
                        self.rx_map[p.src_ip].append(p.data)
            elif p.packet_type == self.ADDR_REQ:
                with self.mutex1:
                    if self.ADDR_LIST.get(p.src_ip) is None:
                        self.ADDR_LIST[p.src_ip] = p.src_mac
                if self.ip == p.dest_ip:
                    p = Packet(p.seq, self.ADDR_RESP, self.mac, p.src_mac, \
                                p.dest_ip, p.src_ip, 'None')
                    self.device.send_packet(p.encode(), remoteAddr=p.dest_mac)
                    print(f"INFO: received a ADDR_REQ packet, send ADDR_RESP {p}")
                else:
                    continue
            elif p.packet_type == self.ADDR_RESP:
                print(f"INFO: received a ADDR_RESP packet, send all the pending data")
                with self.mutex1:
                    self.ADDR_LIST[p.src_ip] = p.src_mac
                with self.mutex3:
                    self.update_timer_map(self.ADDR_REQ, seq=p.seq, packet=None, op=1)
                self.send_pending_data(p.src_ip)
            elif p.packet_type == self.ACK:
                print(f"INFO: received a ACK packet for seq:[{p.seq}]")
                with self.mutex3:
                    self.update_timer_map(self.P2P, seq=p.seq, packet=None, op=1)
            elif (p.seq + 256 - self.recv_seq.get(p.src_mac)) % 256 != 1:
                seq = (self.recv_seq.get(p.src_mac) + 1) % 256
                p = Packet(seq, self.RT_REQ, self.mac, p.src_mac, \
                           p.dest_ip, p.src_ip, 'None')
                if self.timer_map_contains(self.RT_REQ, seq):
                    continue
                self.device.send_packet(p.encode(), remoteAddr=p.dest_mac)
                print(f"INFO: received a disorder packet, send RT_REQ {p}")
                with self.mutex3:
                    self.update_timer_map(self.RT_REQ, 0, packet=p, op=0)
            else:
                with self.mutex2:
                    if self.rx_map.get(p.src_ip) is None:
                        self.rx_map[p.src_ip] = [p.data]
                    else:
                        self.rx_map[p.src_ip].append(p.data)
                self.recv_seq[p.src_mac] = p.seq
                if p.packet_type == self.P2P:
                    p = Packet(p.seq, self.ACK, p.dest_mac, p.src_mac, \
                            p.dest_ip, p.src_ip, 'None')
                    self.device.send_packet(p.encode(), remoteAddr=p.dest_mac)
                    print(f"INFO: received a P2P packet, send ACK {p}")
                else:
                    print(f"INFO: received a BCST packet")



    def timer(self):
        while self.started:
            with self.mutex3:
                for key in self.timer_map:
                    for item in self.timer_map[key]:
                        item[1] -= 1
                        if item[1] < 0:
                            self.rt_queue.put(item[2])
            time.sleep(1)

    
    def retransmission(self):
        while self.started:
            if not self.rt_queue.empty():
                p = self.rt_queue.get()
                if p.dest_mac == self.BCST_ADDR:
                    self.device.send_packet(p.encode())
                else:
                    self.device.send_packet(p.encode(), remoteAddr=p.dest_mac)
                print(f"INFO: send rt {p}")
                with self.mutex3:
                    self.update_timer_map(p.packet_type, seq=p.seq, packet=None, op=2)
                    

    def start_recv(self):
        self.started = True
        recv_th = threading.Thread(target=self.recv, args=())
        recv_th.start()
        handle_th = threading.Thread(target=self.handler, args=())
        handle_th.start()
        timer_th = threading.Thread(target=self.timer, args=())
        timer_th.start()
        rt_th = threading.Thread(target=self.retransmission, args=())
        rt_th.start()


    def stop_recv(self):
        self.started = False


    def get_data(self, dest_ip='0.0.0.0'):
        with self.mutex2:
            try:
                return self.rx_map.get(dest_ip).pop(0)
            except:
                print("The rx queue is empty")
                return None
    
   
    def update_timer_map(self, key, seq, packet, op):
        if op == 0:
            print(f'INFO: add element [{key}][{packet.seq}] to timer_map')
            if self.timer_map.get(key) is None:
                self.timer_map[key] = [[packet.seq, self.timeout, packet]]
            else:
                self.timer_map[key].append([packet.seq, self.timeout, packet])
            return
        index = 0
        for i in range(len(self.timer_map[key])):
            if self.timer_map[key][i][0] == seq:
                index = i
                break
        if op == 1:
            print(f'INFO: remove element [{key}][{self.timer_map[key][index][0]}] from timer_map')
            self.timer_map[key].pop(index)
        elif op == 2:
            print(f'INFO: update element [{key}][{self.timer_map[key][index][0]}]')
            self.timer_map[key][index][1] = self.timeout


    def timer_map_contains(self, key, seq) -> bool:
        with self.mutex3:
            if self.timer_map.get(key) is None:
                return False
            for i in range(len(self.timer_map.get(key))):
                if self.timer_map[key][i][0] == seq:
                    return True
            return False