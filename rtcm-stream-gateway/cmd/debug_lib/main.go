package main

import (
	"encoding/hex"
	"fmt"

	"github.com/go-gnss/rtcm/rtcm3"
)

func main() {
	arp := rtcm3.AntennaReferencePoint{
		ReferenceStationId:        1,
		ItrfRealizationYear:       20,
		GpsIndicator:              true,
		GlonassIndicator:          false,
		GalileoIndicator:          false,
		ReferenceStationIndicator: true,
		ReferencePointX:           -2691181000,
		SingleReceiverOscilator:   false,
		Reserved:                  false,
		ReferencePointY:           -4292232000,
		QuarterCycleIndicator:     0,
		ReferencePointZ:           3855132000,
	}

	msg1005 := rtcm3.Message1005{
		AbstractMessage:       rtcm3.AbstractMessage{MessageNumber: 1005},
		AntennaReferencePoint: arp,
	}
	msg1006 := rtcm3.Message1006{
		AbstractMessage:       rtcm3.AbstractMessage{MessageNumber: 1006},
		AntennaReferencePoint: arp,
		AntennaHeight:         150,
	}

	fmt.Printf("Message1005:\n")
	payload1005 := msg1005.Serialize()
	fmt.Printf("  Payload: %s (%d bytes)\n", hex.EncodeToString(payload1005), len(payload1005))
	frame1005 := rtcm3.EncapsulateMessage(msg1005)
	data1005 := frame1005.Serialize()
	fmt.Printf("  Frame:   %s (%d bytes)\n", hex.EncodeToString(data1005), len(data1005))
	fmt.Printf("  Length field: %d\n", frame1005.Length)

	fmt.Printf("\nMessage1006:\n")
	payload1006 := msg1006.Serialize()
	fmt.Printf("  Payload: %s (%d bytes)\n", hex.EncodeToString(payload1006), len(payload1006))
	frame1006 := rtcm3.EncapsulateMessage(msg1006)
	data1006 := frame1006.Serialize()
	fmt.Printf("  Frame:   %s (%d bytes)\n", hex.EncodeToString(data1006), len(data1006))
	fmt.Printf("  Length field: %d\n", frame1006.Length)

	fmt.Printf("\nHeader bytes:\n")
	fmt.Printf("  1005: %s\n", hex.EncodeToString(data1005[:3]))
	fmt.Printf("  1006: %s\n", hex.EncodeToString(data1006[:3]))
}
