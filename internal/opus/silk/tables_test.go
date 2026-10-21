package silk

import "testing"

// The tables keep the sizes the C declares, and every iCDF table is
// non-increasing and ends with 0, as ICDF needs to stop.
func TestTables(t *testing.T) {
	sizes := []struct {
		name      string
		got, want int
	}{
		{"silk_gain_iCDF", len(gainICDF) * len(gainICDF[0]), 3 * nLevelsQGain / 8},
		{"silk_delta_gain_iCDF", len(deltaGainICDF), maxDeltaGainQuant - minDeltaGainQuant + 1},
		{"silk_pitch_lag_iCDF", len(pitchLagICDF), 2 * (pitchEstMaxLagMs - pitchEstMinLagMs)},
		{"silk_pulses_per_block_iCDF", len(pulsesPerBlockICDF) * len(pulsesPerBlockICDF[0]), nRateLevels * (silkMaxPulses + 2)},
		{"silk_rate_levels_iCDF", len(rateLevelsICDF) * len(rateLevelsICDF[0]), 2 * (nRateLevels - 1)},
		{"silk_shell_code_table_offsets", len(shellCodeTableOffsets), silkMaxPulses + 1},
		{"silk_stereo_pred_quant_Q13", len(stereoPredQuantQ13), stereoQuantTabSize},
		{"silk_LSFCosTab_FIX_Q12", len(lsfCosTabFIXQ12), lsfCosTabSzFIX + 1},
		{"silk_CB_lags_stage2", len(cbLagsStage2), peMaxNbSubfr * peNbCbksStage2Ext},
		{"silk_CB_lags_stage3", len(cbLagsStage3), peMaxNbSubfr * peNbCbksStage3Max},
		{"silk_CB_lags_stage2_10_ms", len(cbLagsStage210Ms), peMaxNbSubfr / 2 * peNbCbksStage210Ms},
		{"silk_CB_lags_stage3_10_ms", len(cbLagsStage310Ms), peMaxNbSubfr / 2 * peNbCbksStage310Ms},
		{"silk_NLSF_CB1_NB_MB_Q8", len(nlsfCBNBMB.cb1NLSFQ8), int(nlsfCBNBMB.nVectors) * int(nlsfCBNBMB.order)},
		{"silk_NLSF_CB1_Wght_Q9", len(nlsfCBNBMB.cb1WghtQ9), int(nlsfCBNBMB.nVectors) * int(nlsfCBNBMB.order)},
		{"silk_NLSF_CB2_SELECT_NB_MB", len(nlsfCBNBMB.ecSel), int(nlsfCBNBMB.nVectors) * int(nlsfCBNBMB.order) / 2},
		{"silk_NLSF_DELTA_MIN_NB_MB_Q15", len(nlsfCBNBMB.deltaMinQ15), int(nlsfCBNBMB.order) + 1},
		{"silk_NLSF_PRED_NB_MB_Q8", len(nlsfCBNBMB.predQ8), 2 * (int(nlsfCBNBMB.order) - 1)},
		{"silk_NLSF_CB1_WB_Q8", len(nlsfCBWB.cb1NLSFQ8), int(nlsfCBWB.nVectors) * int(nlsfCBWB.order)},
		{"silk_NLSF_CB1_WB_Wght_Q9", len(nlsfCBWB.cb1WghtQ9), int(nlsfCBWB.nVectors) * int(nlsfCBWB.order)},
		{"silk_NLSF_CB2_SELECT_WB", len(nlsfCBWB.ecSel), int(nlsfCBWB.nVectors) * int(nlsfCBWB.order) / 2},
		{"silk_NLSF_DELTA_MIN_WB_Q15", len(nlsfCBWB.deltaMinQ15), int(nlsfCBWB.order) + 1},
		{"silk_NLSF_PRED_WB_Q8", len(nlsfCBWB.predQ8), 2 * (int(nlsfCBWB.order) - 1)},
		{"silk_resampler_frac_FIR_12", len(resamplerFracFIR12) * len(resamplerFracFIR12[0]), 12 * resamplerOrderFIR12 / 2},
		{"silk_Resampler_3_4_COEFS", len(resampler34Coefs), 2 + 3*resamplerDownOrderFIR0/2},
		{"silk_Resampler_2_3_COEFS", len(resampler23Coefs), 2 + 2*resamplerDownOrderFIR0/2},
		{"silk_Resampler_1_2_COEFS", len(resampler12Coefs), 2 + resamplerDownOrderFIR1/2},
		{"silk_Resampler_1_6_COEFS", len(resampler16Coefs), 2 + resamplerDownOrderFIR2/2},
	}
	for _, s := range sizes {
		if s.got != s.want {
			t.Errorf("%s: %d values, want %d", s.name, s.got, s.want)
		}
	}

	icdfs := map[string][]uint8{
		"silk_delta_gain_iCDF":                deltaGainICDF[:],
		"silk_pitch_lag_iCDF":                 pitchLagICDF[:],
		"silk_pitch_delta_iCDF":               pitchDeltaICDF[:],
		"silk_pitch_contour_iCDF":             pitchContourICDF[:],
		"silk_pitch_contour_NB_iCDF":          pitchContourNBICDF[:],
		"silk_pitch_contour_10_ms_iCDF":       pitchContour10MsICDF[:],
		"silk_pitch_contour_10_ms_NB_iCDF":    pitchContour10MsNBICDF[:],
		"silk_lsb_iCDF":                       lsbICDF[:],
		"silk_uniform3_iCDF":                  uniform3ICDF[:],
		"silk_uniform4_iCDF":                  uniform4ICDF[:],
		"silk_uniform5_iCDF":                  uniform5ICDF[:],
		"silk_uniform6_iCDF":                  uniform6ICDF[:],
		"silk_uniform8_iCDF":                  uniform8ICDF[:],
		"silk_NLSF_EXT_iCDF":                  nlsfExtICDF[:],
		"silk_LTP_per_index_iCDF":             ltpPerIndexICDF[:],
		"silk_LTPscale_iCDF":                  ltpScaleICDF[:],
		"silk_type_offset_VAD_iCDF":           typeOffsetVADICDF[:],
		"silk_type_offset_no_VAD_iCDF":        typeOffsetNoVADICDF[:],
		"silk_stereo_pred_joint_iCDF":         stereoPredJointICDF[:],
		"silk_stereo_only_code_mid_iCDF":      stereoOnlyCodeMidICDF[:],
		"silk_NLSF_interpolation_factor_iCDF": nlsfInterpolationFactorICDF[:],
		"silk_LBRR_flags_2_iCDF":              lbrrFlagsICDFPtr[0],
		"silk_LBRR_flags_3_iCDF":              lbrrFlagsICDFPtr[1],
	}
	for i := range gainICDF {
		icdfs["silk_gain_iCDF row"+string(rune('0'+i))] = gainICDF[i][:]
	}
	for i := range pulsesPerBlockICDF {
		icdfs["silk_pulses_per_block_iCDF row"+string(rune('0'+i))] = pulsesPerBlockICDF[i][:]
	}
	for i := range rateLevelsICDF {
		icdfs["silk_rate_levels_iCDF row"+string(rune('0'+i))] = rateLevelsICDF[i][:]
	}
	for i, tab := range ltpGainICDFPtrs {
		icdfs["silk_LTP_gain_iCDF_"+string(rune('0'+i))] = tab
	}
	for name, tab := range icdfs {
		checkICDF(t, name, tab)
	}
	// Tables of several iCDFs: each one starts after the previous one's 0.
	for name, tab := range map[string][]uint8{
		"silk_NLSF_CB1_iCDF_NB_MB": nlsfCB1ICDFNBMB[:],
		"silk_NLSF_CB1_iCDF_WB":    nlsfCB1ICDFWB[:],
		"silk_NLSF_CB2_iCDF_NB_MB": nlsfCB2ICDFNBMB[:],
		"silk_NLSF_CB2_iCDF_WB":    nlsfCB2ICDFWB[:],
		"silk_shell_code_table0":   shellCodeTable0[:],
		"silk_shell_code_table1":   shellCodeTable1[:],
		"silk_shell_code_table2":   shellCodeTable2[:],
		"silk_shell_code_table3":   shellCodeTable3[:],
	} {
		start := 0
		for i, v := range tab {
			if v == 0 {
				checkICDF(t, name, tab[start:i+1])
				start = i + 1
			}
		}
		if start != len(tab) {
			t.Errorf("%s does not end with 0", name)
		}
	}
	// silk_shell_code_table_offsets point at the iCDF of each pulse count.
	for p := 1; p <= silkMaxPulses; p++ {
		off := shellCodeTableOffsets[p]
		if n := int(shellCodeTableOffsets[p]) - int(shellCodeTableOffsets[p-1]); p > 1 && n != p {
			t.Errorf("shell code table offset %d: %d entries before it, want %d", p, n, p)
		}
		if shellCodeTable0[int(off)+p] != 0 {
			t.Errorf("shell code table for %d pulses does not end at its last symbol", p)
		}
	}
}

func checkICDF(t *testing.T, name string, tab []uint8) {
	t.Helper()
	if len(tab) == 0 || tab[len(tab)-1] != 0 {
		t.Errorf("%s does not end with 0", name)
		return
	}
	for i := 1; i < len(tab); i++ {
		if tab[i] > tab[i-1] {
			t.Errorf("%s increases at %d", name, i)
			return
		}
	}
}
