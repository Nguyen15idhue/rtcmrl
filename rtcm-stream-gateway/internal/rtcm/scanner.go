package rtcm

import (
	"encoding/binary"
	"math"

	"github.com/bamiaux/iobit"
)

const (
	rtcm3Preamble = 0xD3
)

type Frame struct {
	Length  uint16
	Payload []byte
	Crc     uint32
}

type Scanner struct {
	buf    []byte
	synced bool
}

func NewScanner() *Scanner {
	return &Scanner{buf: make([]byte, 0, 4096)}
}

func (s *Scanner) Push(chunk []byte) [][]byte {
	if len(chunk) == 0 {
		return nil
	}
	s.buf = append(s.buf, chunk...)
	frames := make([][]byte, 0, 8)

	for {
		if !s.synced {
			i := findByte(s.buf, rtcm3Preamble)
			if i < 0 {
				if len(s.buf) > 8192 {
					s.buf = s.buf[len(s.buf)-4096:]
				}
				return frames
			}
			s.buf = s.buf[i:]
			s.synced = true
		}

		if len(s.buf) < 6 {
			return frames
		}

		r := iobit.NewReader(s.buf)
		preamble := r.Uint8(8)
		if preamble != rtcm3Preamble {
			s.buf = s.buf[1:]
			s.synced = false
			continue
		}
		r.Skip(6)
		length := r.Uint16(10)

		if length > 1023 {
			s.buf = s.buf[1:]
			s.synced = false
			continue
		}

		totalBytes := 3 + int(length) + 3
		if len(s.buf) < totalBytes {
			return frames
		}

		data := s.buf[:totalBytes]
		crcData := data[:totalBytes-3]
		gotCRC := uint32(data[totalBytes-3])<<16 | uint32(data[totalBytes-2])<<8 | uint32(data[totalBytes-1])
		calcCRC := CRC24Q(crcData)

		if calcCRC == gotCRC {
			copyFrame := make([]byte, totalBytes)
			copy(copyFrame, data)
			frames = append(frames, copyFrame)
			s.buf = s.buf[totalBytes:]
			continue
		}

		s.buf = s.buf[1:]
		s.synced = false
	}
}

func findByte(buf []byte, want byte) int {
	for i, b := range buf {
		if b == want {
			return i
		}
	}
	return -1
}

func ParseFrame(data []byte) (Frame, bool) {
	if len(data) < 6 || data[0] != rtcm3Preamble {
		return Frame{}, false
	}
	r := iobit.NewReader(data)
	r.Skip(8)
	r.Skip(6)
	length := r.Uint16(10)

	total := 3 + int(length) + 3
	if len(data) < total {
		return Frame{}, false
	}

	crcData := data[:total-3]
	gotCRC := uint32(data[total-3])<<16 | uint32(data[total-2])<<8 | uint32(data[total-1])

	if CRC24Q(crcData) != gotCRC {
		return Frame{}, false
	}

	return Frame{
		Length:  length,
		Payload: data[3 : 3+length],
		Crc:     gotCRC,
	}, true
}

func MessageNumber(payload []byte) uint16 {
	if len(payload) < 2 {
		return 0
	}
	return (uint16(payload[0])<<4 | uint16(payload[1])>>4) & 0xFFF
}

func RefStationID(payload []byte) uint16 {
	if len(payload) < 3 {
		return 0
	}
	return ((uint16(payload[1]) & 0x0F) << 8) | uint16(payload[2])
}

func Encapsulate(payload []byte) []byte {
	hdr := make([]byte, 3)
	hdr[0] = rtcm3Preamble
	hdr[1] = byte(len(payload) >> 8)
	hdr[2] = byte(len(payload))

	data := append(hdr, payload...)
	crc := CRC24Q(data)
	return append(data,
		byte(crc>>16),
		byte(crc>>8),
		byte(crc),
	)
}

func BuildFrame(msgType uint16, stationID uint16, body []byte) []byte {
	msg := make([]byte, 2+len(body))
	msg[0] = byte(msgType >> 4)
	msg[1] = byte((msgType&0x0F)<<4) | byte(stationID>>8)
	msg[2] = byte(stationID)
	if len(body) > 0 {
		copy(msg[3:], body)
	}
	return Encapsulate(msg)
}

func BuildMsg1005(stationID uint16, x, y, z, height float64) []byte {
	coords := make([]byte, 12)
	binary.LittleEndian.PutUint64(coords[0:8], math.Float64bits(x))
	binary.LittleEndian.PutUint64(coords[8:16], math.Float64bits(y))
	binary.LittleEndian.PutUint64(coords[16:24], math.Float64bits(z))
	copy(coords[0:12], coords[0:12])

	h := make([]byte, 2)
	binary.LittleEndian.PutUint16(h, uint16(math.Float64bits(height)&0xFFFF))

	body := make([]byte, 14)
	copy(body[0:12], coords[0:12])
	copy(body[12:14], h)

	return BuildFrame(1005, stationID, body)
}
