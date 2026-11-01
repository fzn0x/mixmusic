// Port of silk/sort.c (silk_insertion_sort_increasing_all_values_int16, the part the decoder uses).

package silk

// insertionSortIncreasingAllValuesInt16 is
// silk_insertion_sort_increasing_all_values_int16: insertion sort, fast for
// already almost sorted arrays.
func insertionSortIncreasingAllValuesInt16(a []int16, L int32) {
	var value int32
	var i, j int32

	// Sort vector elements by value, increasing order
	for i = 1; i < L; i++ {
		value = int32(a[i])
		for j = i - 1; (j >= 0) && (value < int32(a[j])); j-- {
			a[j+1] = a[j] // Shift value
		}
		a[j+1] = int16(value) // Write value
	}
}
