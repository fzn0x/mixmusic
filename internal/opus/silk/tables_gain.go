// Port of silk/tables_gain.c, its values copied verbatim by a script.

package silk

// gainICDF is silk_gain_iCDF.
var gainICDF = [3][8]uint8{
	{
		224, 112, 44, 15, 3, 2, 1, 0,
	},
	{
		254, 237, 192, 132, 70, 23, 4, 0,
	},
	{
		255, 252, 226, 155, 61, 11, 2, 0,
	},
}

// deltaGainICDF is silk_delta_gain_iCDF.
var deltaGainICDF = [41]uint8{
	250, 245, 234, 203, 71, 50, 42, 38,
	35, 33, 31, 29, 28, 27, 26, 25,
	24, 23, 22, 21, 20, 19, 18, 17,
	16, 15, 14, 13, 12, 11, 10, 9,
	8, 7, 6, 5, 4, 3, 2, 1,
	0,
}
