package wifi

import (
	"errors"

	"github.com/tarm/serial"
)

const (
	MAX_FRAME_DATA_LEN = 1024
	RESPONSE_TIMEOUT   = 3
	BCST_MAC           = "1111111111"
)

type Writer struct {
	Port *serial.Port
	Seq  int
}

func NewWriter(port *serial.Port) *Writer {
	return &Writer{
		Port: port,
		Seq:  0,
	}
}

func (writer *Writer) GetNodes(r *Reader) ([][]byte, error) {
	resps := make([][]byte, 0)
	frame := writer.GenerateSearchRequest()
	writer.Port.Write(frame)
	timeout := 8
	for {
		resp, err := r.GetResp(timeout, frame[FRAME_SEQ_OFFSET])
		if err != nil {
			return resps, nil
		} else {
			timeout = RESPONSE_TIMEOUT
			if writer.isAddressResponseValid(resp, frame) {
				resps = append(resps, resp[ADDR_PAYLOAD_OFFSET:len(resp)-1])
			} else {
				return nil, errors.New("received response is invalid")
			}
		}
	}
}

func (writer *Writer) GenerateTransmitRequest(srcMac string, dstMac string, data []byte) []byte {
	length := 1 + 1 + 1 + 5 + 5 + len(data) + 1
	frame := make([]byte, 6)
	frame[DELIMITER_OFFSET] = DELIMITER
	frame[LENGTH_HIGH_OFFSET] = byte(length >> 8)
	frame[LENGTH_LOW_OFFSET] = byte(length & 0xFF)
	frame[FRAME_SEQ_OFFSET] = byte(writer.Seq)
	frame[FRAME_TYPE_OFFSET] = TRANSMIT_REQUEST
	if dstMac == BCST_MAC {
		frame[TRANSMIT_TYPE_OFFSET] = FRAME_BCST
	} else {
		frame[TRANSMIT_TYPE_OFFSET] = FRAME_P2P
	}
	frame = append(frame, StrToBytes(srcMac)...)
	frame = append(frame, StrToBytes(dstMac)...)
	frame = append(frame, data...)
	frame = append(frame, calcCheckSum(frame))
	writer.Seq = (writer.Seq + 1) % 256
	return frame
}

func (writer *Writer) GenerateSearchRequest() []byte {
	length := 3
	frame := make([]byte, 5)
	frame[DELIMITER_OFFSET] = DELIMITER
	frame[LENGTH_HIGH_OFFSET] = byte(length >> 8)
	frame[LENGTH_LOW_OFFSET] = byte(length & 0xFF)
	frame[FRAME_SEQ_OFFSET] = byte(writer.Seq)
	frame[FRAME_TYPE_OFFSET] = SEARCH_REQUEST
	frame = append(frame, calcCheckSum(frame))
	writer.Seq = (writer.Seq + 1) % 256
	return frame
}

func (writer *Writer) GenerateAddressRequest() []byte {
	length := 3
	frame := make([]byte, 5)
	frame[DELIMITER_OFFSET] = DELIMITER
	frame[LENGTH_HIGH_OFFSET] = byte(length >> 8)
	frame[LENGTH_LOW_OFFSET] = byte(length & 0xFF)
	frame[FRAME_SEQ_OFFSET] = byte(writer.Seq)
	frame[FRAME_TYPE_OFFSET] = ADDRESS_REQUEST
	frame = append(frame, calcCheckSum(frame))
	writer.Seq = (writer.Seq + 1) % 256
	return frame
}

func (writer *Writer) SendPacketWithResponse(srcMac string, dstMac string, data []byte, reader *Reader) error {
	if len(data) > MAX_FRAME_DATA_LEN {
		return errors.New("data too long")
	}
	frame := writer.GenerateTransmitRequest(srcMac, dstMac, data)
	writer.Port.Write(frame)
	resp, err := reader.GetStatus(RESPONSE_TIMEOUT, frame[FRAME_SEQ_OFFSET])
	if err != nil {
		return err
	}
	if writer.isTransmitResultValid(resp, frame) {
		// fmt.Println("send a frame: 0")
		// print(frame)
		return nil
	} else {
		return errors.New("received frame is invalid")
	}
}

func (writer *Writer) SendAddrReqWithResponse(reader *Reader) ([]byte, error) {
	frame := writer.GenerateAddressRequest()
	writer.Port.Write(frame)
	resp, err := reader.GetResp(RESPONSE_TIMEOUT, frame[FRAME_SEQ_OFFSET])
	if err != nil {
		return nil, err
	}
	if writer.isAddressResponseValid(resp, frame) {
		return resp[ADDR_PAYLOAD_OFFSET : len(resp)-1], nil
	} else {
		return nil, errors.New("received response is invalid")
	}
}

func (writer *Writer) isTransmitResultValid(resp []byte, frame []byte) bool {
	if !validate(resp) {
		return false
	}
	if resp[FRAME_SEQ_OFFSET] != frame[FRAME_SEQ_OFFSET] {
		return false
	}
	if resp[RESULT_STATUS_OFFSET] != RESULT_OK {
		return false
	}
	return true
}

func (writer *Writer) isAddressResponseValid(resp []byte, frame []byte) bool {
	if !validate(resp) {
		return false
	}
	if resp[FRAME_SEQ_OFFSET] != frame[FRAME_SEQ_OFFSET] {
		return false
	}
	return true
}
