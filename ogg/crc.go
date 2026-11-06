package ogg

// crcTable is of the CRC-32 of pages: the polynomial 0x04C11DB7, from
// zero, unreflected.
var crcTable = func() (t [256]uint32) {
	for i := range 256 {
		c := uint32(i) << 24
		for range 8 {
			if c&0x80000000 != 0 {
				c = c<<1 ^ 0x04C11DB7
			} else {
				c <<= 1
			}
		}
		t[i] = c
	}
	return
}()

// update adds b to the checksum c.
func update(c uint32, b []byte) uint32 {
	for _, v := range b {
		c = c<<8 ^ crcTable[byte(c>>24)^v]
	}
	return c
}
