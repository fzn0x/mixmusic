// Port of silk/NLSF_unpack.c.

package silk

// nlsfUnpack is silk_NLSF_unpack: unpacks the indices to entropy tables,
// ecIx, and the LSF predictor, predQ8, of vector CB1Index of the first LSF
// codebook.
func nlsfUnpack(ecIx []int16, predQ8 []uint8, psNLSFCB *nlsfCBStruct, CB1Index int32) {
	var entry uint8
	order := int32(psNLSFCB.order)

	ecSelPtr := psNLSFCB.ecSel[CB1Index*order/2:]
	for i := int32(0); i < order; i += 2 {
		entry = ecSelPtr[0]
		ecSelPtr = ecSelPtr[1:]
		ecIx[i] = int16(smulbb(int32(entry>>1)&7, 2*nlsfQuantMaxAmplitude+1))
		predQ8[i] = psNLSFCB.predQ8[i+int32(entry&1)*(order-1)]
		ecIx[i+1] = int16(smulbb(int32(entry>>5)&7, 2*nlsfQuantMaxAmplitude+1))
		predQ8[i+1] = psNLSFCB.predQ8[i+int32((entry>>4)&1)*(order-1)+1]
	}
}
