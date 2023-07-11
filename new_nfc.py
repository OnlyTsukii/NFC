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

    BCST_ADDR  = 'FFFFFFFFFFFFFFFF'

    DEV_LIST = {
        '1027:24577': 'Xbee'
    }

    ADDR_LIST = {}

    def __init__(self, ip: str):
        self.port_list = []
        self.dev_type = None
        self.dev_port = None
        self.device = None
        self.mac = 0
        self.seq = -1
        self.ip = ip
        self.pending_data = {}
        self.tx_map = {}
        self.rx_map = {}
        self.rx_queue = queue.Queue()
        self.started = False

        self.mutex = threading.Lock()
        self.mutex1 = threading.Lock()
        self.mutex2 = threading.Lock()


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
            

    def send(self, data: bytes, dest_ip='0.0.0.0'):
        try:
            data = data.decode('UTF-8')
            self.seq = (self.seq + 1) % 256
            if dest_ip == '0.0.0.0':
                p = Packet(self.seq, self.BCST, self.mac, \
                           self.BCST_ADDR, self.ip, dest_ip, data)
                with self.mutex:
                    self.tx_map[self.seq] = p
                self.send_packet(p)
                print(f"INFO: send BCST {p}")
            else:
                with self.mutex1:
                    dest_mac = self.ADDR_LIST.get(dest_ip)
                if dest_mac is not None:
                    p = Packet(self.seq, self.P2P, self.mac, \
                                dest_mac, self.ip, dest_ip, data)
                    with self.mutex:
                        self.tx_map[self.seq] = p
                    self.send_packet(p, dest_mac)
                    print(f"INFO: send P2P {p}")
                else:
                    p = Packet(self.seq, self.ADDR_REQ, self.mac, \
                               self.BCST_ADDR, self.ip, dest_ip, 'None')
                    if self.pending_data.get(dest_ip) is None:   
                        with self.mutex1:
                            self.pending_data[dest_ip] = [[p.seq, data]]
                        self.send_packet(p)
                        print(f"INFO: send ADDR_REQ {p}")
                    else:
                        with self.mutex1:
                            self.pending_data[dest_ip].append([p.seq, data])

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
                    self.send_packet(p, dest_mac)
                    print(f"INFO: send pending data {p}")
        except Exception as e:
            print(f"Error occurred when sending pending data: {e}")

    
    def send_packet(self, packet, dest_mac=None):
        while True:
            if self.device.send_packet(packet.encode(), remoteAddr=dest_mac):
                break
    
    
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
            
            if self.rx_map_contains(p.src_ip, p.seq, p.packet_type):
                print(f"INFO: received a duplicate {p}")
                continue
            else:
                with self.mutex2:
                    if self.rx_map.get(p.src_ip) is None:
                        self.rx_map[p.src_ip] = [p]
                    else:
                        self.rx_map[p.src_ip].append(p)
                if p.packet_type == self.ADDR_REQ:
                    with self.mutex1:
                        if self.ADDR_LIST.get(p.src_ip) is None:
                            self.ADDR_LIST[p.src_ip] = p.src_mac
                    if self.ip == p.dest_ip:
                        p = Packet(p.seq, self.ADDR_RESP, self.mac, p.src_mac, \
                                    p.dest_ip, p.src_ip, 'None')
                        self.send_packet(p, p.dest_mac)
                        print(f"INFO: received a ADDR_REQ packet, send ADDR_RESP {p}")
                    else:
                        continue
                elif p.packet_type == self.ADDR_RESP:
                    print(f"INFO: received a ADDR_RESP packet, send all the pending data")
                    with self.mutex1:
                        self.ADDR_LIST[p.src_ip] = p.src_mac
                    self.send_pending_data(p.src_ip)
                elif p.packet_type == self.P2P:
                    print(f"INFO: received a P2P {p}")
                else:
                    print(f"INFO: received a BCST {p}")
                    

    def start_recv(self):
        self.started = True
        recv_th = threading.Thread(target=self.recv, args=())
        recv_th.start()
        handle_th = threading.Thread(target=self.handler, args=())
        handle_th.start()


    def stop_recv(self):
        self.started = False


    def get_data(self, dest_ip='0.0.0.0'):
        with self.mutex2:
            try:
                return self.rx_map.get(dest_ip).pop(0)
            except:
                print("The rx queue is empty")
                return None


    def rx_map_contains(self, key, seq, packet_type) -> bool:
        with self.mutex2:
            if self.rx_map.get(key) is None or len(self.rx_map.get(key)) == 0:
                return False
            return self.rx_map[key][-1].seq == seq and \
                self.rx_map[key][-1].packet_type == packet_type