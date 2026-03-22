package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/your-org/rtcm-stream-gateway/internal/rtcm"
)

var (
	host          = flag.String("host", "127.0.0.1", "Gateway host")
	port          = flag.Int("port", 12101, "Gateway port")
	stations      = flag.Int("stations", 5, "Number of virtual stations")
	interval      = flag.Int("interval", 1000, "Interval between frames (ms)")
	singleStation = flag.Int("station", 0, "Use single station ID (0 = multiple)")
)

func main() {
	flag.Parse()

	addr := fmt.Sprintf("%s:%d", *host, *port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to connect to %s: %v", addr, err)
	}
	defer conn.Close()

	log.Printf("Connected to %s, generating RTCM data for %d stations", addr, *stations)

	var stationIDs []int
	if *singleStation > 0 {
		stationIDs = []int{*singleStation}
	} else {
		stationIDs = []int{1, 2, 3, 1001, 2001}
		if *stations > len(stationIDs) {
			for i := len(stationIDs); i < *stations; i++ {
				stationIDs = append(stationIDs, 3000+i)
			}
		}
	}

	ticker := time.NewTicker(time.Duration(*interval) * time.Millisecond)
	defer ticker.Stop()

	frameCount := 0
	for {
		select {
		case <-ticker.C:
			for _, sid := range stationIDs {
				frame := rtcm.BuildFrame(1006, uint16(sid), generateBody(sid, frameCount))
				if _, err := conn.Write(frame); err != nil {
					log.Printf("Write error for station %d: %v", sid, err)
					return
				}
			}
			frameCount++
			if frameCount%100 == 0 {
				log.Printf("Sent %d frames for %d stations", frameCount, len(stationIDs))
			}
		}
	}
}

func generateBody(stationID, frameNum int) []byte {
	body := make([]byte, 14)
	x := -2697745.0 + float64(stationID)*0.1
	y := 5019520.0 + float64(stationID)*0.1
	z := 2564650.0 + float64(stationID)*0.1

	copy(body, encodeCoord(x, y, z))

	return body
}

func encodeCoord(x, y, z float64) []byte {
	b := make([]byte, 14)

	sx := int32(x / 0.0001)
	sy := int32(y / 0.0001)
	sz := int32(z / 0.0001)

	b[0] = byte((sx >> 24) & 0xFF)
	b[1] = byte((sx >> 16) & 0xFF)
	b[2] = byte((sx >> 8) & 0xFF)
	b[3] = byte(sx & 0xFF)
	b[4] = byte((sy >> 24) & 0xFF)
	b[5] = byte((sy >> 16) & 0xFF)
	b[6] = byte((sy >> 8) & 0xFF)
	b[7] = byte(sy & 0xFF)
	b[8] = byte((sz >> 24) & 0xFF)
	b[9] = byte((sz >> 16) & 0xFF)
	b[10] = byte((sz >> 8) & 0xFF)
	b[11] = byte(sz & 0xFF)
	b[12] = 0x00
	b[13] = 0x00

	return b
}
