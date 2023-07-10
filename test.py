
list = [0, 1, 2]

print(list.pop(0))


# PACKET TEST

# import struct

# class Packet:
#     def __init__(self, seq: int, packet_type: int, src_mac: str, \
#                  dest_mac: str, src_ip: str, dest_ip: str, data) -> None:
#         self.seq = seq
#         self.packet_type = packet_type
#         self.src_mac = src_mac
#         self.dest_mac = dest_mac
#         self.src_ip = src_ip
#         self.dest_ip = dest_ip
#         self.data = data


#     def encode(self) -> bytes:
#         src_ip = ip2hex(self.src_ip).encode('UTF-8')
#         dest_ip = ip2hex(self.dest_ip).encode('UTF-8')
#         src_mac = self.src_mac.encode('UTF-8')
#         dest_mac = self.dest_mac.encode('UTF-8')
#         data_bytes = self.data.encode('UTF-8')

#         packet_data = struct.pack('>BB16s16s8s8s', self.seq, self.packet_type, \
#                                   src_mac, dest_mac, src_ip, dest_ip) + data_bytes

#         return packet_data
    

#     @classmethod
#     def decode(cls, data: bytes) -> 'Packet':
#         seq, packet_type, src_mac, dest_mac, src_ip, \
#             dest_ip = struct.unpack('>BB16s16s8s8s', data[:50])
#         data = data[50:].decode('UTF-8')

#         return cls(seq, packet_type, src_mac.decode('UTF-8'), dest_mac.decode('UTF-8'), \
#                    hex2ip(src_ip.decode('UTF-8')), hex2ip(dest_ip.decode('UTF-8')), data)
    

#     def __repr__(self) -> str:
#         return f"Packet(seq={self.seq}, packet_type={self.packet_type}, src_mac={self.src_mac}, dest_mac={self.dest_mac}, src_ip={self.src_ip}, dest_ip={self.dest_ip}, data={self.data})"
    
    

# def ip2hex(ip: str) -> str:
#     list = ip.split('.')
#     hex_list = [format(int(item), '02x') for item in list]
#     return ''.join(hex_list)


# def hex2ip(hex: str) -> str:
#     ip_list = []
#     while len(hex) > 0:
#         ip_list.append(str(int(hex[:2], 16)))
#         ip_list.append('.')
#         hex = hex[2:]
#     return ''.join(ip_list[:-1])


# p = Packet(0, 0, '1212121212121212', '3434343434343434', '192.168.0.1', '192.168.0.2', 'sdfsfvsdvf')
# temp = p.encode()
# print(temp)
# print(p.decode(temp))



# START&STOP TEST

# import threading
# import time

# class Test:
#     def __init__(self):
#         self.started = False

#     def test(self):
#         while self.started:
#             print("alive")
#             time.sleep(1)

#     def start(self):
#         self.started = True
#         test_th = threading.Thread(target=self.test, args=())
#         test_th.start()

#     def stop(self):
#         self.started = False

# test = Test()
# test.start()
# time.sleep(5)
# test.stop()




# TIMER TEST

# import time

# timer_map = {}

# timer_map[0] = [[0, 30, 'packet1']]
# timer_map[0].append([1, 30, 'packet2'])
# timer_map[1] = [[2, 30, 'packet3']]
# timer_map[1].append([3, 30, 'packet4'])
# timer_map[1].append([4, 30, 'packet5'])


# while True:
#     for key in timer_map:
#         for item in timer_map[key]:
#             print(item)
#             item[1] -= 1
#             if item[1] < 0:
#                 print('enqueue')
#     time.sleep(1)
