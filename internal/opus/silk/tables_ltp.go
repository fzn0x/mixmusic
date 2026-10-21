// Port of silk/tables_LTP.c, its values copied verbatim by a script.

package silk

// ltpPerIndexICDF is silk_LTP_per_index_iCDF.
var ltpPerIndexICDF = [3]uint8{
	179, 99, 0,
}

// ltpGainICDF0 is silk_LTP_gain_iCDF_0.
var ltpGainICDF0 = [8]uint8{
	71, 56, 43, 30, 21, 12, 6, 0,
}

// ltpGainICDF1 is silk_LTP_gain_iCDF_1.
var ltpGainICDF1 = [16]uint8{
	199, 165, 144, 124, 109, 96, 84, 71,
	61, 51, 42, 32, 23, 15, 8, 0,
}

// ltpGainICDF2 is silk_LTP_gain_iCDF_2.
var ltpGainICDF2 = [32]uint8{
	241, 225, 211, 199, 187, 175, 164, 153,
	142, 132, 123, 114, 105, 96, 88, 80,
	72, 64, 57, 50, 44, 38, 33, 29,
	24, 20, 16, 12, 9, 5, 2, 0,
}

// ltpGainICDFPtrs is silk_LTP_gain_iCDF_ptrs.
var ltpGainICDFPtrs = [nbLTPCbks][]uint8{
	ltpGainICDF0[:],
	ltpGainICDF1[:],
	ltpGainICDF2[:],
}

// ltpGainVQ0 is silk_LTP_gain_vq_0 ([8][5]), flattened as silk_LTP_vq_ptrs_Q7 reads it.
var ltpGainVQ0 = [40]int8{
	4, 6, 24, 7, 5,
	0, 0, 2, 0, 0,
	12, 28, 41, 13, -4,
	-9, 15, 42, 25, 14,
	1, -2, 62, 41, -9,
	-10, 37, 65, -4, 3,
	-6, 4, 66, 7, -8,
	16, 14, 38, -3, 33,
}

// ltpGainVQ1 is silk_LTP_gain_vq_1 ([16][5]), flattened.
var ltpGainVQ1 = [80]int8{
	13, 22, 39, 23, 12,
	-1, 36, 64, 27, -6,
	-7, 10, 55, 43, 17,
	1, 1, 8, 1, 1,
	6, -11, 74, 53, -9,
	-12, 55, 76, -12, 8,
	-3, 3, 93, 27, -4,
	26, 39, 59, 3, -8,
	2, 0, 77, 11, 9,
	-8, 22, 44, -6, 7,
	40, 9, 26, 3, 9,
	-7, 20, 101, -7, 4,
	3, -8, 42, 26, 0,
	-15, 33, 68, 2, 23,
	-2, 55, 46, -2, 15,
	3, -1, 21, 16, 41,
}

// ltpGainVQ2 is silk_LTP_gain_vq_2 ([32][5]), flattened.
var ltpGainVQ2 = [160]int8{
	-6, 27, 61, 39, 5,
	-11, 42, 88, 4, 1,
	-2, 60, 65, 6, -4,
	-1, -5, 73, 56, 1,
	-9, 19, 94, 29, -9,
	0, 12, 99, 6, 4,
	8, -19, 102, 46, -13,
	3, 2, 13, 3, 2,
	9, -21, 84, 72, -18,
	-11, 46, 104, -22, 8,
	18, 38, 48, 23, 0,
	-16, 70, 83, -21, 11,
	5, -11, 117, 22, -8,
	-6, 23, 117, -12, 3,
	3, -8, 95, 28, 4,
	-10, 15, 77, 60, -15,
	-1, 4, 124, 2, -4,
	3, 38, 84, 24, -25,
	2, 13, 42, 13, 31,
	21, -4, 56, 46, -1,
	-1, 35, 79, -13, 19,
	-7, 65, 88, -9, -14,
	20, 4, 81, 49, -29,
	20, 0, 75, 3, -17,
	5, -9, 44, 92, -8,
	1, -3, 22, 69, 31,
	-6, 95, 41, -12, 5,
	39, 67, 16, -4, 1,
	0, -6, 120, 55, -36,
	-13, 44, 122, 4, -24,
	81, 5, 11, 3, 7,
	2, 0, 9, 10, 88,
}

// ltpVQPtrsQ7 is silk_LTP_vq_ptrs_Q7.
var ltpVQPtrsQ7 = [nbLTPCbks][]int8{
	ltpGainVQ0[:],
	ltpGainVQ1[:],
	ltpGainVQ2[:],
}
