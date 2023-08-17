package vxbee

import (
	"errors"
	"log"
)

const (
	HEAD_SIZE         = 3
	MAX_FRAG_DATA_LEN = 238
)

type Fragment struct {
	No    int
	Seq   int
	Total int
	Data  []byte
}

func NewFragment(no, seq, total int, data []byte) (*Fragment, error) {
	if no > 255 || seq > 255 || total > 255 {
		return nil, errors.New("no, seq, total must be less than 256")
	} else if len(data) > MAX_FRAG_DATA_LEN {
		return nil, errors.New("data too long")
	}
	p := &Fragment{
		No:    no,
		Seq:   seq,
		Total: total,
		Data:  data,
	}
	return p, nil
}

func (p *Fragment) Encode() []byte {
	no := byte(p.No)
	seq := byte(p.Seq)
	total := byte(p.Total)
	data := p.Data

	fragment := make([]byte, 3+len(data))
	fragment[0] = no
	fragment[1] = seq
	fragment[2] = total
	copy(fragment[3:], data)
	return fragment
}

func DecodeFragment(data []byte) (*Fragment, error) {
	if len(data) < HEAD_SIZE {
		return nil, errors.New("invalid packet data")
	}
	no := int(data[0])
	seq := int(data[1])
	total := int(data[2])
	p := &Fragment{
		No:    no,
		Seq:   seq,
		Total: total,
		Data:  data[HEAD_SIZE:],
	}
	return p, nil
}

func GetFragments(no int, data []byte) ([]*Fragment, error) {
	if len(data) < HEAD_SIZE {
		return nil, errors.New("payload too small")
	}
	fragments := []*Fragment{}
	dataList := splitData(data, MAX_FRAG_DATA_LEN)
	if len(dataList) > 256 {
		return nil, errors.New("data too long")
	}
	for i, data := range dataList {
		p, err := NewFragment(no, i, len(dataList), data)
		if err != nil {
			log.Fatal(err)
		}
		fragments = append(fragments, p)
	}
	return fragments, nil
}

func splitData(data []byte, chunkSize int) [][]byte {
	var chunks [][]byte
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunks = append(chunks, data[i:end])
	}
	return chunks
}
