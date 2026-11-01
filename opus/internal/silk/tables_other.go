// Port of silk/tables_other.c, its values copied verbatim by a script.

package silk

// stereoPredQuantQ13 is silk_stereo_pred_quant_Q13.
var stereoPredQuantQ13 = [16]int16{
	-13732, -10050, -8266, -7526, -6500, -5000, -2950, -820,
	820, 2950, 5000, 6500, 7526, 8266, 10050, 13732,
}

// stereoPredJointICDF is silk_stereo_pred_joint_iCDF.
var stereoPredJointICDF = [25]uint8{
	249, 247, 246, 245, 244,
	234, 210, 202, 201, 200,
	197, 174, 82, 59, 56,
	55, 54, 46, 22, 12,
	11, 10, 9, 7, 0,
}

// stereoOnlyCodeMidICDF is silk_stereo_only_code_mid_iCDF.
var stereoOnlyCodeMidICDF = [2]uint8{
	64, 0,
}

// lbrrFlags2ICDF is silk_LBRR_flags_2_iCDF.
var lbrrFlags2ICDF = [3]uint8{
	203, 150, 0,
}

// lbrrFlags3ICDF is silk_LBRR_flags_3_iCDF.
var lbrrFlags3ICDF = [7]uint8{
	215, 195, 166, 125, 110, 82, 0,
}

// lbrrFlagsICDFPtr is silk_LBRR_flags_iCDF_ptr.
var lbrrFlagsICDFPtr = [2][]uint8{
	lbrrFlags2ICDF[:],
	lbrrFlags3ICDF[:],
}

// lsbICDF is silk_lsb_iCDF.
var lsbICDF = [2]uint8{
	120, 0,
}

// ltpScaleICDF is silk_LTPscale_iCDF.
var ltpScaleICDF = [3]uint8{
	128, 64, 0,
}

// typeOffsetVADICDF is silk_type_offset_VAD_iCDF.
var typeOffsetVADICDF = [4]uint8{
	232, 158, 10, 0,
}

// typeOffsetNoVADICDF is silk_type_offset_no_VAD_iCDF.
var typeOffsetNoVADICDF = [2]uint8{
	230, 0,
}

// nlsfInterpolationFactorICDF is silk_NLSF_interpolation_factor_iCDF.
var nlsfInterpolationFactorICDF = [5]uint8{
	243, 221, 192, 181, 0,
}

// quantizationOffsetsQ10 is silk_Quantization_Offsets_Q10.
var quantizationOffsetsQ10 = [2][2]int16{
	{offsetUVLQ10, offsetUVHQ10}, {offsetVLQ10, offsetVHQ10},
}

// ltpScalesTableQ14 is silk_LTPScales_table_Q14.
var ltpScalesTableQ14 = [3]int16{
	15565, 12288, 8192,
}

// uniform3ICDF is silk_uniform3_iCDF.
var uniform3ICDF = [3]uint8{
	171, 85, 0,
}

// uniform4ICDF is silk_uniform4_iCDF.
var uniform4ICDF = [4]uint8{
	192, 128, 64, 0,
}

// uniform5ICDF is silk_uniform5_iCDF.
var uniform5ICDF = [5]uint8{
	205, 154, 102, 51, 0,
}

// uniform6ICDF is silk_uniform6_iCDF.
var uniform6ICDF = [6]uint8{
	213, 171, 128, 85, 43, 0,
}

// uniform8ICDF is silk_uniform8_iCDF.
var uniform8ICDF = [8]uint8{
	224, 192, 160, 128, 96, 64, 32, 0,
}

// nlsfExtICDF is silk_NLSF_EXT_iCDF.
var nlsfExtICDF = [7]uint8{
	100, 40, 16, 7, 3, 1, 0,
}
