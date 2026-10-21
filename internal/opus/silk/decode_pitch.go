// Port of silk/decode_pitch.c.

package silk

// decodePitch is silk_decode_pitch: the pitch lags of the nbSubfr subframes.
func decodePitch(lagIndex int16, contourIndex int8, pitchLags []int32, FsKHz, nbSubfr int32) {
	var lag, k, minLag, maxLag, cbkSize int32
	var lagCBPtr []int8

	if FsKHz == 8 {
		if nbSubfr == peMaxNbSubfr {
			lagCBPtr = cbLagsStage2[:]
			cbkSize = peNbCbksStage2Ext
		} else {
			lagCBPtr = cbLagsStage210Ms[:]
			cbkSize = peNbCbksStage210Ms
		}
	} else {
		if nbSubfr == peMaxNbSubfr {
			lagCBPtr = cbLagsStage3[:]
			cbkSize = peNbCbksStage3Max
		} else {
			lagCBPtr = cbLagsStage310Ms[:]
			cbkSize = peNbCbksStage310Ms
		}
	}

	minLag = smulbb(peMinLagMs, FsKHz)
	maxLag = smulbb(peMaxLagMs, FsKHz)
	lag = minLag + int32(lagIndex)

	for k = 0; k < nbSubfr; k++ {
		pitchLags[k] = lag + int32(lagCBPtr[k*cbkSize+int32(contourIndex)]) // matrix_ptr( Lag_CB_ptr, k, contourIndex, cbk_size )
		pitchLags[k] = limit(pitchLags[k], minLag, maxLag)
	}
}
