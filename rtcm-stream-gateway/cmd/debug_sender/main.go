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

	frame := rtcm3.EncapsulateMessage(msg1005)
	data := frame.Serialize()

	fmt.Printf("Serialized frame (%d bytes): %s\n", len(data), hex.EncodeToString(data))

	payload := data[3 : len(data)-3]
	fmt.Printf("Payload (%d bytes): %s\n", len(payload), hex.EncodeToString(payload))
	fmt.Printf("Msg type from payload[0:2]: %d\n", (uint16(payload[0])<<4|uint16(payload[1])>>4)&0xFFF)
	fmt.Printf("Station ID from payload[1:3]: %d\n", ((uint16(payload[1])&0x0F)<<8)|uint16(payload[2]))
}
