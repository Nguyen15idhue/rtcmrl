package rtcm

import (
	"fmt"
	"hash/crc32"
)

// MessageType returns RTCM3 message type (12 bits) from a validated frame.
func MessageType(frame []byte) (int, bool) {
	if len(frame) < 6 || frame[0] != 0xD3 {
		return 0, false
	}
	payloadLen := int(frame[1]&0x03)<<8 | int(frame[2])
	if payloadLen < 2 || len(frame) < 3+payloadLen+3 {
		return 0, false
	}
	msg := (int(frame[3]) << 4) | int(frame[4]>>4)
	msg &= 0x0FFF
	return msg, true
}

// StationID returns 12-bit reference station ID when available.
// For most observation/station messages, this is stored in bits 12..23
// right after the 12-bit message type.
func StationID(frame []byte) (int, bool) {
	if len(frame) < 6 || frame[0] != 0xD3 {
		return 0, false
	}
	payloadLen := int(frame[1]&0x03)<<8 | int(frame[2])
	if payloadLen < 3 || len(frame) < 3+payloadLen+3 {
		return 0, false
	}
	id := (int(frame[4]&0x0F) << 8) | int(frame[5])
	id &= 0x0FFF
	if id == 0 {
		return 0, false
	}
	return id, true
}

// StationFingerprint returns a stable fingerprint for a station from
// static metadata messages. This is used to avoid Station ID collisions.
//
// Uses payload CRC from messages that are typically static per station:
// - 1005/1006: station coordinates
// - 1033: receiver/antenna descriptor
func StationFingerprint(frame []byte) (string, bool) {
	msg, ok := MessageType(frame)
	if !ok {
		return "", false
	}
	if msg != 1005 && msg != 1006 && msg != 1033 {
		return "", false
	}
	payloadLen := int(frame[1]&0x03)<<8 | int(frame[2])
	if payloadLen <= 0 || len(frame) < 3+payloadLen+3 {
		return "", false
	}
	payload := frame[3 : 3+payloadLen]
	h := crc32.ChecksumIEEE(payload)
	return fmt.Sprintf("%08x", h), true
}
