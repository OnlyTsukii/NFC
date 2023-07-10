import logging
from typing import List


class Packet:
    HEAD_SIZE = 3
    MAX_PACKETS_SIZE = 256

    def __init__(self, no: int, seq: int, total: int, data: bytes) -> None:
        if no.bit_length() > 8 or seq.bit_length() > 8 or total.bit_length() > 8:
            raise ValueError(f"dev, no, seq, total must be less than 256")
        self.seq = seq

        self.no = no
        self.total = total
        self.data = data

    def encode(self) -> bytes:
        return self.no.to_bytes(1, byteorder='big') + \
            self.seq.to_bytes(1, byteorder='big') + \
            self.total.to_bytes(1, byteorder='big') + self.data

    @classmethod
    def decode(cls, data: bytes) -> 'Packet':
        no = int.from_bytes(data[0:1], byteorder='big')
        seq = int.from_bytes(data[1:2], byteorder='big')
        total = int.from_bytes(data[2:3], byteorder='big')
        data = data[Packet.HEAD_SIZE:]
        return cls(no, seq, total, data)

    def __repr__(self) -> str:
        return f"Packet(no={self.no}, seq={self.seq}, total={self.total}, data={self.data})"


def get_packets(no: int, data: bytes, payload=256) -> List[Packet]:
    payload = payload - Packet.HEAD_SIZE
    if payload < 0:
        raise ValueError("payload too small")
    packets = []
    data_list = [data[i:i + payload] for i in range(0, len(data), payload)]
    if len(data_list) > Packet.MAX_PACKETS_SIZE:
        raise ValueError("data too long")
    for i, data in enumerate(data_list):
        packets.append(Packet(no, i, len(data_list), data))
    logging.debug("size of packets: %s", len(packets))
    return packets


def get_data(packets: List[Packet]) -> bytes:
    data = b''
    for p in packets:
        data += p.data
    return data
