package rtcm

func makeCRC24QTable() [256]uint32 {
	var table [256]uint32
	for i := 0; i < 256; i++ {
		crc := uint32(i) << 16
		for j := 0; j < 8; j++ {
			crc <<= 1
			if crc&0x1000000 != 0 {
				crc ^= 0x1864CFB
			}
		}
		table[i] = crc & 0xFFFFFF
	}
	return table
}

var crc24qTable = makeCRC24QTable()

func CRC24Q(data []byte) uint32 {
	var crc uint32
	for _, b := range data {
		idx := byte((crc>>16)^uint32(b))
		crc = ((crc << 8) ^ crc24qTable[idx]) & 0xFFFFFF
	}
	return crc
}
