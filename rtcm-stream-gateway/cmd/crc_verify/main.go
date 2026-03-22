package main

import (
	"encoding/hex"
	"fmt"

	"github.com/your-org/rtcm-stream-gateway/internal/rtcm"
)

func main() {
	frame, _ := hex.DecodeString("d3000d3ed000004132d687e3d70a3d00000029ac76")
	fmt.Printf("Frame: %s (%d bytes)\n", hex.EncodeToString(frame), len(frame))
	fmt.Printf("Bytes 0-2 (header): %x\n", frame[:3])
	fmt.Printf("Byte 1: %02x &0xFC=%02x (should be 00)\n", frame[1], frame[1]&0xFC)

	payloadLen := (int(frame[1]&0x03) << 8) | int(frame[2])
	total := 3 + payloadLen + 3
	fmt.Printf("Payload len: %d, total: %d, len(frame): %d\n", payloadLen, total, len(frame))

	gotCRC := uint32(frame[total-3])<<16 | uint32(frame[total-2])<<8 | uint32(frame[total-1])
	fmt.Printf("Got CRC from bytes %d-%d: %06x\n", total-3, total-1, gotCRC)

	crcData := frame[:total-3]
	fmt.Printf("CRC data: %s (%d bytes)\n", hex.EncodeToString(crcData), len(crcData))
	goCRC := rtcm.CRC24Q(crcData)
	fmt.Printf("Go CRC24Q(data): %06x\n", goCRC)
	fmt.Printf("Match: %v\n", gotCRC == goCRC)

	// Scanner test
	s := rtcm.NewScanner()
	frames := s.Push(frame)
	fmt.Printf("Scanner extracted: %d frames\n", len(frames))
}
