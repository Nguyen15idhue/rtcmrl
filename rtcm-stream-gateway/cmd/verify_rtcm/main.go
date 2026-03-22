package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/your-org/rtcm-stream-gateway/internal/rtcm"
)

func main() {
	data, _ := hex.DecodeString("d300133ed00003b5964465b88637bbb352b774e1e5794c9492")
	fmt.Printf("Frame: %s (%d bytes)\n", hex.EncodeToString(data), len(data))

	f, ok := rtcm.ParseFrame(data)
	if !ok {
		fmt.Println("FAILED to parse frame")
		os.Exit(1)
	}
	fmt.Printf("OK: Length=%d, PayloadLen=%d, CRC=%06x\n", len(data), f.Length, f.Crc)
	fmt.Printf("Payload: %s\n", hex.EncodeToString(f.Payload))
	fmt.Printf("MessageNumber: %d\n", rtcm.MessageNumber(f.Payload))
	fmt.Printf("RefStationID: %d\n", rtcm.RefStationID(f.Payload))

	s := rtcm.NewScanner()
	frames := s.Push(data)
	fmt.Printf("Scanner extracted: %d frames\n", len(frames))
	if len(frames) > 0 {
		fmt.Printf("First frame: %s\n", hex.EncodeToString(frames[0]))
	}
}
