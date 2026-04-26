package envelope

import "encoding/binary"

const (
	FlagCompressed byte = 1 << 0
	FlagEndStream  byte = 1 << 1
)

func Encode(flags byte, payload []byte) []byte {
	buf := make([]byte, 5+len(payload))
	buf[0] = flags
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(payload)))
	copy(buf[5:], payload)
	return buf
}

func Decode(frame []byte) (flags byte, payload []byte, err error) {
	if len(frame) < 5 {
		return 0, nil, fmt.Errorf("envelope: frame too short (%d bytes)", len(frame))
	}
	flags = frame[0]
	length := binary.BigEndian.Uint32(frame[1:5])
	if uint32(len(frame)-5) < length {
		return 0, nil, fmt.Errorf("envelope: expected %d payload bytes, got %d", length, len(frame)-5)
	}
	payload = frame[5 : 5+length]
	return flags, payload, nil
}
