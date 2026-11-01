// Port of silk/pitch_est_tables.c, its values copied verbatim by a script.

package silk

// cbLagsStage210Ms is silk_CB_lags_stage2_10_ms ([2][3]), flattened for matrix_ptr.
var cbLagsStage210Ms = [6]int8{
	0, 1, 0,
	0, 0, 1,
}

// cbLagsStage310Ms is silk_CB_lags_stage3_10_ms ([2][12]), flattened.
var cbLagsStage310Ms = [24]int8{
	0, 0, 1, -1, 1, -1, 2, -2, 2, -2, 3, -3,
	0, 1, 0, 1, -1, 2, -1, 2, -2, 3, -2, 3,
}

// cbLagsStage2 is silk_CB_lags_stage2 ([4][11]), flattened.
var cbLagsStage2 = [44]int8{
	0, 2, -1, -1, -1, 0, 0, 1, 1, 0, 1,
	0, 1, 0, 0, 0, 0, 0, 1, 0, 0, 0,
	0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 0,
	0, -1, 2, 1, 0, 1, 1, 0, 0, -1, -1,
}

// cbLagsStage3 is silk_CB_lags_stage3 ([4][34]), flattened.
var cbLagsStage3 = [136]int8{
	0, 0, 1, -1, 0, 1, -1, 0, -1, 1, -2, 2, -2, -2, 2, -3, 2, 3, -3, -4, 3, -4, 4, 4, -5, 5, -6, -5, 6, -7, 6, 5, 8, -9,
	0, 0, 1, 0, 0, 0, 0, 0, 0, 0, -1, 1, 0, 0, 1, -1, 0, 1, -1, -1, 1, -1, 2, 1, -1, 2, -2, -2, 2, -2, 2, 2, 3, -3,
	0, 1, 0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 0, 1, -1, 1, 0, 0, 2, 1, -1, 2, -1, -1, 2, -1, 2, 2, -1, 3, -2, -2, -2, 3,
	0, 1, 0, 0, 1, 0, 1, -1, 2, -1, 2, -1, 2, 3, -2, 3, -2, -2, 4, 4, -3, 5, -3, -4, 6, -4, 6, 5, -5, 8, -6, -5, -7, 9,
}
