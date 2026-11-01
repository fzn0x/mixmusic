package silk

import (
	"math"
	"math/rand/v2"
	"testing"
)

// Values worked out by hand from the C macros.
func TestMacros(t *testing.T) {
	tests := []struct {
		name      string
		got, want int64
	}{
		{"smulwb(65536, 3)", int64(smulwb(65536, 3)), 3},
		{"smulwb(-65536, 3)", int64(smulwb(-65536, 3)), -3},
		{"smulwb(1<<20, -1)", int64(smulwb(1<<20, -1)), -16},
		{"smulwb(100, 0x18000) uses int16(b)", int64(smulwb(100, 0x18000)), -50},
		{"smlawb(10, 65536, 2)", int64(smlawb(10, 65536, 2)), 12},
		{"smulbb(0x10002, 3) uses int16(a)", int64(smulbb(0x10002, 3)), 6},
		{"smlabb(1, -2, 3)", int64(smlabb(1, -2, 3)), -5},
		{"smulww(1<<16, 1<<16)", int64(smulww(1<<16, 1<<16)), 1 << 16},
		{"smulww(-3, 65536)", int64(smulww(-3, 65536)), -3},
		{"smlaww(1, 1<<20, 1<<12)", int64(smlaww(1, 1<<20, 1<<12)), 1 + 1<<16},
		{"smultt(3<<16, -2<<16)", int64(smultt(3<<16, -2<<16)), -6},
		{"smmul(1<<30, 1<<30)", int64(smmul(1<<30, 1<<30)), 1 << 28},
		{"mlaOvflw(1, 1<<16, 1<<16) wraps", int64(mlaOvflw(1, 1<<16, 1<<16)), 1},
		{"rshiftRound(5, 1)", int64(rshiftRound(5, 1)), 3},
		{"rshiftRound(-5, 1)", int64(rshiftRound(-5, 1)), -2},
		{"rshiftRound(6, 2)", int64(rshiftRound(6, 2)), 2},
		{"rshiftRound(-6, 2)", int64(rshiftRound(-6, 2)), -1},
		{"rshiftRound64(-6, 2)", rshiftRound64(-6, 2), -1},
		{"addSat32(max, 1)", int64(addSat32(math.MaxInt32, 1)), math.MaxInt32},
		{"addSat32(min, -1)", int64(addSat32(math.MinInt32, -1)), math.MinInt32},
		{"addSat32(max, min)", int64(addSat32(math.MaxInt32, math.MinInt32)), -1},
		{"subSat32(min, 1)", int64(subSat32(math.MinInt32, 1)), math.MinInt32},
		{"subSat32(0, min)", int64(subSat32(0, math.MinInt32)), math.MaxInt32},
		{"subSat32(-1, max)", int64(subSat32(-1, math.MaxInt32)), math.MinInt32},
		{"lshiftSat32(5, 4)", int64(lshiftSat32(5, 4)), 80},
		{"lshiftSat32(1<<30, 1)", int64(lshiftSat32(1<<30, 1)), 2147483646},
		{"lshiftSat32(-(1<<30), 1)", int64(lshiftSat32(-(1 << 30), 1)), math.MinInt32},
		{"limit(5, 0, 3)", int64(limit(5, 0, 3)), 3},
		{"limit(5, 3, 0)", int64(limit(5, 3, 0)), 3},
		{"limit(-1, 3, 0)", int64(limit(-1, 3, 0)), 0},
		{"limit(2, 3, 0)", int64(limit(2, 3, 0)), 2},
		{"sat16(40000)", int64(sat16(40000)), 32767},
		{"sat16(-40000)", int64(sat16(-40000)), -32768},
		{"addSat16(32767, 1)", int64(addSat16(32767, 1)), 32767},
		{"silkRAND(0)", int64(silkRAND(0)), 907633515},
		{"silkRAND(1)", int64(silkRAND(1)), 907633515 + 196314165},
		{"silkRAND(-1)", int64(silkRAND(-1)), 907633515 - 196314165},
		{"clz32(0)", int64(clz32(0)), 32},
		{"clz32(1)", int64(clz32(1)), 31},
		{"clz32(-1)", int64(clz32(-1)), 0},
		{"clz32(0x8000)", int64(clz32(0x8000)), 16},
		{"ror32(1, 1)", int64(ror32(1, 1)), math.MinInt32},
		{"ror32(0x12345678, 8)", int64(ror32(0x12345678, 8)), 0x78123456},
		{"ror32(0x12345678, -8)", int64(ror32(0x12345678, -8)), 0x34567812},
		// 100 = 0b1100100: 25 leading zeros, then 1100100 -> frac 0b1001000.
		{"clzFrac(100) lz", int64(first(clzFrac(100))), 25},
		{"clzFrac(100) frac", int64(second(clzFrac(100))), 72},
		{"sqrtApprox(65536)", int64(sqrtApprox(65536)), 256},
		{"sqrtApprox(32768)", int64(sqrtApprox(32768)), 180},
		{"sqrtApprox(100)", int64(sqrtApprox(100)), 9},
		{"sqrtApprox(0)", int64(sqrtApprox(0)), 0},
		{"div32VarQ(65536, 65536, 16)", int64(div32VarQ(65536, 65536, 16)), 65535},
		{"inverse32VarQ(65536, 46)", int64(inverse32VarQ(65536, 46)), 1073741823},
		{"log2lin(-1)", int64(log2lin(-1)), 0},
		{"log2lin(0)", int64(log2lin(0)), 1},
		{"log2lin(128)", int64(log2lin(128)), 2},
		{"log2lin(1088)", int64(log2lin(1088)), 362},   // 2^8.5 = 362.04
		{"log2lin(2112)", int64(log2lin(2112)), 92672}, // 2^16.5 = 92681.9
		{"log2lin(3967)", int64(log2lin(3967)), math.MaxInt32},
		{"int16ToRes(-32768)*32768", int64(int16ToRes(-32768) * 32768), -32768},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.want)
		}
	}
}

func first(a, _ int32) int32  { return a }
func second(_, b int32) int32 { return b }

// The OPUS_FAST_INT64 forms of the macros used here equal their 32-bit
// forms of macros.h, and the saturating ones equal a clamp.
func TestMacroForms(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	smulwb32 := func(a, b int32) int32 { // without OPUS_FAST_INT64
		return (a>>16)*int32(int16(b)) + ((a&0xFFFF)*int32(int16(b)))>>16
	}
	smulww32 := func(a, b int32) int32 { // silk_MLA(silk_SMULWB(a, b), a, silk_RSHIFT_ROUND(b, 16))
		return smulwb32(a, b) + a*rshiftRound(b, 16)
	}
	clamp := func(x int64) int32 { return int32(min(max(x, math.MinInt32), math.MaxInt32)) }
	for i := 0; i < 1000000; i++ {
		a, b, c := int32(r.Uint32()), int32(r.Uint32()), int32(r.Uint32())
		if i%4 == 0 { // small values too
			a >>= r.IntN(32)
			b >>= r.IntN(32)
		}
		if got, want := smulwb(a, b), smulwb32(a, b); got != want {
			t.Fatalf("smulwb(%d, %d) = %d, want %d", a, b, got, want)
		}
		if got, want := smlawb(c, a, b), c+smulwb32(a, b); got != want {
			t.Fatalf("smlawb(%d, %d, %d) = %d, want %d", c, a, b, got, want)
		}
		if got, want := smulww(a, b), smulww32(a, b); got != want {
			t.Fatalf("smulww(%d, %d) = %d, want %d", a, b, got, want)
		}
		if got, want := smlaww(c, a, b), c+smulww32(a, b); got != want {
			t.Fatalf("smlaww(%d, %d, %d) = %d, want %d", c, a, b, got, want)
		}
		if got, want := addSat32(a, b), clamp(int64(a)+int64(b)); got != want {
			t.Fatalf("addSat32(%d, %d) = %d, want %d", a, b, got, want)
		}
		if got, want := subSat32(a, b), clamp(int64(a)-int64(b)); got != want {
			t.Fatalf("subSat32(%d, %d) = %d, want %d", a, b, got, want)
		}
	}
}
