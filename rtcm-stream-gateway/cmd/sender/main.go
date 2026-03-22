package main

import (
	"log"
	"net"
	"time"

	"github.com/go-gnss/rtcm/rtcm3"
)

type StationConfig struct {
	ID      uint16
	X, Y, Z float64
	Height  float64
}

func main() {
	addr := "127.0.0.1:12101"
	stations := []StationConfig{
		{1, -2691181.0, -4292232.0, 3855132.0, 1.5},
		{2, -2691000.0, -4292000.0, 3855200.0, 2.0},
		{3, -2690800.0, -4291800.0, 3855300.0, 1.0},
		{1001, -2690700.0, -4291700.0, 3855400.0, 1.8},
		{2001, -2690600.0, -4291600.0, 3855500.0, 2.2},
	}

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		log.Fatalf("Failed to connect to %s: %v", addr, err)
	}
	defer conn.Close()
	log.Printf("Connected to %s, sending RTCM frames...", addr)

	fps := 5.0
	interval := time.Duration(float64(time.Second) / fps)

	for {
		for _, s := range stations {
			frames := buildStationFrames(s)
			for _, frame := range frames {
				_, err := conn.Write(frame)
				if err != nil {
					log.Printf("Write error: %v", err)
					return
				}
			}
		}
		time.Sleep(interval)
	}
}

func buildStationFrames(s StationConfig) [][]byte {
	arp := rtcm3.AntennaReferencePoint{
		ReferenceStationId:        s.ID,
		ItrfRealizationYear:       20,
		GpsIndicator:              true,
		GlonassIndicator:          false,
		GalileoIndicator:          false,
		ReferenceStationIndicator: true,
		ReferencePointX:           int64(s.X * 1000),
		SingleReceiverOscilator:   false,
		Reserved:                  false,
		ReferencePointY:           int64(s.Y * 1000),
		QuarterCycleIndicator:     0,
		ReferencePointZ:           int64(s.Z * 1000),
	}

	msg1005 := rtcm3.Message1005{
		AbstractMessage:       rtcm3.AbstractMessage{MessageNumber: 1005},
		AntennaReferencePoint: arp,
	}
	msg1006 := rtcm3.Message1006{
		AbstractMessage:       rtcm3.AbstractMessage{MessageNumber: 1006},
		AntennaReferencePoint: arp,
		AntennaHeight:         uint16(s.Height * 100),
	}

	var frames [][]byte
	for _, msg := range []rtcm3.Message{msg1005, msg1006} {
		frame := rtcm3.EncapsulateMessage(msg)
		frames = append(frames, frame.Serialize())
	}
	return frames

	return frames
}
