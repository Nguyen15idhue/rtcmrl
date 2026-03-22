package rtcm

import (
	"fmt"
	"hash/crc32"

	"github.com/bamiaux/iobit"
)

func MessageType(frame []byte) (int, bool) {
	if len(frame) < 6 || frame[0] != rtcm3Preamble {
		return 0, false
	}
	r := iobit.NewReader(frame)
	r.Skip(8)
	r.Skip(6)
	length := r.Uint16(10)

	if int(length)+3 > len(frame) {
		return 0, false
	}
	payload := frame[3 : 3+length]
	if len(payload) < 2 {
		return 0, false
	}
	msg := (int(payload[0])<<4 | int(payload[1])>>4) & 0xFFF
	return msg, true
}

func StationID(frame []byte) (int, bool) {
	if len(frame) < 6 || frame[0] != rtcm3Preamble {
		return 0, false
	}
	r := iobit.NewReader(frame)
	r.Skip(8)
	r.Skip(6)
	length := r.Uint16(10)

	if int(length)+3 > len(frame) || length < 3 {
		return 0, false
	}
	payload := frame[3 : 3+length]
	id := ((int(payload[1]) & 0x0F) << 8) | int(payload[2])
	id &= 0xFFF
	if id == 0 {
		return 0, false
	}
	return id, true
}

func StationFingerprint(frame []byte) (string, bool) {
	msg, ok := MessageType(frame)
	if !ok {
		return "", false
	}
	if msg != 1005 && msg != 1006 && msg != 1033 {
		return "", false
	}
	if len(frame) < 6 || frame[0] != rtcm3Preamble {
		return "", false
	}
	r := iobit.NewReader(frame)
	r.Skip(8)
	r.Skip(6)
	length := r.Uint16(10)

	if int(length)+3 > len(frame) || length == 0 {
		return "", false
	}
	payload := frame[3 : 3+length]
	h := crc32.ChecksumIEEE(payload)
	return fmt.Sprintf("%08x", h), true
}
