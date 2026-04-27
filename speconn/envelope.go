package speconn

import (
	"encoding/binary"
	"fmt"
)

const (
	FlagCompressed byte = 0x01
	FlagEndStream  byte = 0x02
)

func EncodeEnvelope(flags byte, payload []byte) []byte {
	buf := make([]byte, 5+len(payload))
	buf[0] = flags
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(payload)))
	copy(buf[5:], payload)
	return buf
}

func DecodeEnvelope(data []byte) (flags byte, payload []byte, err error) {
	if len(data) < 5 {
		return 0, nil, fmt.Errorf("speconn: envelope too short (%d bytes)", len(data))
	}
	flags = data[0]
	length := binary.BigEndian.Uint32(data[1:5])
	if uint32(len(data)-5) < length {
		return 0, nil, fmt.Errorf("speconn: expected %d payload bytes, got %d", length, len(data)-5)
	}
	payload = data[5 : 5+length]
	return flags, payload, nil
}
