package main

import (
	"fmt"
)

func main() {
	// Exact bytes from gateway log: d3000d3ed000004132d687e3d70a3d00000029ac76
	// This is the FIRST 21 bytes of the "firstBytes" hex string
	frame := []byte{
		0xd3, 0x00, 0x0d, 0x3e, 0xd0, 0x00, 0x00, 0x41, 0x32, 0xd6,
		0x87, 0xe3, 0xd7, 0x0a, 0x3d, 0x00, 0x00, 0x00, 0x29, 0xac, 0x76,
	}
	fmt.Printf("Frame len: %d\n", len(frame))
	fmt.Printf("frame[0]=%02x frame[1]=%02x frame[2]=%02x\n", frame[0], frame[1], frame[2])
	fmt.Printf("frame[1]&0xFC = %02x\n", frame[1]&0xFC)

	payloadLen := (int(frame[1]&0x03) << 8) | int(frame[2])
	fmt.Printf("payloadLen = %d\n", payloadLen)
	total := 3 + payloadLen + 3
	fmt.Printf("total = %d, len(frame) = %d\n", total, len(frame))

	// Simulate scanner
	buf := frame
	synced := false

	// Step 1: findByte(buf, 0xD3)
	i := -1
	for j := 0; j < len(buf); j++ {
		if buf[j] == 0xD3 {
			i = j
			break
		}
	}
	fmt.Printf("\nStep 1: findByte returned %d\n", i)
	if i >= 0 {
		buf = buf[i:]
		synced = true
		fmt.Printf("  buf now len=%d, synced=%v, buf[0]=%02x buf[1]=%02x buf[2]=%02x\n", len(buf), synced, buf[0], buf[1], buf[2])
	}

	// Step 2: check len >= 6
	fmt.Printf("Step 2: len(buf)=%d >= 6 = %v\n", len(buf), len(buf) >= 6)

	// Step 3: buf[0] == 0xD3
	fmt.Printf("Step 3: buf[0]=%02x == D3 = %v\n", buf[0], buf[0] == 0xD3)

	// Step 4: buf[1]&0xFC == 0
	fmt.Printf("Step 4: buf[1]&0xFC=%02x == 0 = %v\n", buf[1]&0xFC, buf[1]&0xFC == 0)

	// Step 5: payloadLen and total
	fmt.Printf("Step 5: payloadLen=%d, total=%d, len(buf)=%d\n", payloadLen, total, len(buf))
	if len(buf) < total {
		fmt.Printf("  Not enough bytes! Need %d, have %d\n", total, len(buf))
	} else {
		// Step 6: CRC check
		crcData := buf[:total-3]
		fmt.Printf("Step 6: CRC data = %02x (%d bytes)\n", crcData, len(crcData))

		// Compute Go CRC
		crc := uint32(0)
		// CRC24Q table
		var table [256]uint32
		for t := 0; t < 256; t++ {
			ct := uint32(t) << 16
			for j := 0; j < 8; j++ {
				ct <<= 1
				if ct&0x1000000 != 0 {
					ct ^= 0x1864CFB
				}
			}
			table[t] = ct & 0xFFFFFF
		}
		for _, b := range crcData {
			crc = ((crc << 8) ^ table[(crc>>16)^uint32(b)]) & 0xFFFFFF
		}
		gotCRC := uint32(buf[total-3])<<16 | uint32(buf[total-2])<<8 | uint32(buf[total-1])
		fmt.Printf("  Got CRC from bytes %d-%d: %06x\n", total-3, total-1, gotCRC)
		fmt.Printf("  Computed CRC: %06x\n", crc)
		fmt.Printf("  Match: %v\n", crc == gotCRC)
	}
}
