package flac

// The checksums of frames: CRC-8 of the polynomial x^8 + x^2 + x + 1 for
// their headers, and CRC-16 of x^16 + x^15 + x^2 + 1 for them whole, both
// from zero and unreflected.
var crc8Table, crc16Table = func() (t8 [256]uint8, t16 [256]uint16) {
	for i := range 256 {
		c8 := uint8(i)
		c16 := uint16(i) << 8
		for range 8 {
			if c8&0x80 != 0 {
				c8 = c8<<1 ^ 0x07
			} else {
				c8 <<= 1
			}
			if c16&0x8000 != 0 {
				c16 = c16<<1 ^ 0x8005
			} else {
				c16 <<= 1
			}
		}
		t8[i], t16[i] = c8, c16
	}
	return
}()

func crc8(b []byte) uint8 {
	var c uint8
	for _, v := range b {
		c = crc8Table[c^v]
	}
	return c
}

func crc16(b []byte) uint16 {
	var c uint16
	for _, v := range b {
		c = c<<8 ^ crc16Table[uint8(c>>8)^v]
	}
	return c
}
