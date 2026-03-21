package rtcm

// Scanner keeps stream state and yields valid RTCM3 frames.
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
			i := findByte(s.buf, 0xD3)
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
		if s.buf[0] != 0xD3 {
			s.synced = false
			continue
		}
		if s.buf[1]&0xFC != 0 {
			s.buf = s.buf[1:]
			s.synced = false
			continue
		}

		payloadLen := int(s.buf[1]&0x03)<<8 | int(s.buf[2])
		total := 3 + payloadLen + 3
		if total < 6 {
			s.buf = s.buf[1:]
			s.synced = false
			continue
		}
		if len(s.buf) < total {
			return frames
		}

		frame := s.buf[:total]
		gotCRC := uint32(frame[total-3])<<16 | uint32(frame[total-2])<<8 | uint32(frame[total-1])
		if CRC24Q(frame[:total-3]) == gotCRC {
			copyFrame := make([]byte, total)
			copy(copyFrame, frame)
			frames = append(frames, copyFrame)
			s.buf = s.buf[total:]
			continue
		}

		// CRC fail: slide one byte and resync.
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
