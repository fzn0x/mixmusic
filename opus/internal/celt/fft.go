package celt

// Port of celt/kiss_fft.c (opus_fft_impl and its butterflies) for floats.
// The FFT works in place on interleaved complex values: element k is
// f[2k] + i f[2k+1].

// fftImpl is opus_fft_impl.
func fftImpl(st *fftState, f []float32) {
	var fstride [maxFactors]int
	shift := max(st.shift, 0)
	fstride[0] = 1
	l := 0
	var m int
	for {
		p := int(st.factors[2*l])
		m = int(st.factors[2*l+1])
		fstride[l+1] = fstride[l] * p
		l++
		if m == 1 {
			break
		}
	}
	m = int(st.factors[2*l-1])
	for i := l - 1; i >= 0; i-- {
		m2 := 1
		if i != 0 {
			m2 = int(st.factors[2*i-1])
		}
		switch st.factors[2*i] {
		case 2:
			kfBfly2(f, m, fstride[i])
		case 4:
			kfBfly4(f, fstride[i]<<shift, st, m, fstride[i], m2)
		case 3:
			kfBfly3(f, fstride[i]<<shift, st, m, fstride[i], m2)
		case 5:
			kfBfly5(f, fstride[i]<<shift, st, m, fstride[i], m2)
		}
		m = m2
	}
}

func kfBfly2(f []float32, m, n int) {
	const tw = float32(0.7071067812)
	_ = m // m is 4: the radix 2 follows a radix 4
	o := 0
	for i := 0; i < n; i++ {
		// Fout is f[o:], Fout2 is f[o+8:], in floats.
		a, b := o, o+8
		tr, ti := f[b], f[b+1]
		f[b], f[b+1] = f[a]-tr, f[a+1]-ti
		f[a] += tr
		f[a+1] += ti

		tr = (f[b+2] + f[b+3]) * tw
		ti = (f[b+3] - f[b+2]) * tw
		f[b+2], f[b+3] = f[a+2]-tr, f[a+3]-ti
		f[a+2] += tr
		f[a+3] += ti

		tr = f[b+5]
		ti = -f[b+4]
		f[b+4], f[b+5] = f[a+4]-tr, f[a+5]-ti
		f[a+4] += tr
		f[a+5] += ti

		tr = (f[b+7] - f[b+6]) * tw
		ti = -(f[b+7] + f[b+6]) * tw
		f[b+6], f[b+7] = f[a+6]-tr, f[a+7]-ti
		f[a+6] += tr
		f[a+7] += ti
		o += 16
	}
}

func kfBfly4(f []float32, fstride int, st *fftState, m, n, mm int) {
	if m == 1 {
		// All the twiddles are 1.
		o := 0
		for i := 0; i < n; i++ {
			s0r, s0i := f[o]-f[o+4], f[o+1]-f[o+5]
			f[o] += f[o+4]
			f[o+1] += f[o+5]
			s1r, s1i := f[o+2]+f[o+6], f[o+3]+f[o+7]
			f[o+4], f[o+5] = f[o]-s1r, f[o+1]-s1i
			f[o] += s1r
			f[o+1] += s1i
			s1r, s1i = f[o+2]-f[o+6], f[o+3]-f[o+7]
			f[o+2] = s0r + s1i
			f[o+3] = s0i - s1r
			f[o+6] = s0r - s1i
			f[o+7] = s0i + s1r
			o += 8
		}
		return
	}
	tw := st.twiddles
	m2, m3 := 2*m, 3*m
	for i := 0; i < n; i++ {
		k := i * mm // the complex element Fout points at
		t1, t2, t3 := 0, 0, 0
		for j := 0; j < m; j++ {
			a := 2 * k
			b, c, d := 2*(k+m), 2*(k+m2), 2*(k+m3)
			s0r := f[b]*tw[t1].r - f[b+1]*tw[t1].i
			s0i := f[b]*tw[t1].i + f[b+1]*tw[t1].r
			s1r := f[c]*tw[t2].r - f[c+1]*tw[t2].i
			s1i := f[c]*tw[t2].i + f[c+1]*tw[t2].r
			s2r := f[d]*tw[t3].r - f[d+1]*tw[t3].i
			s2i := f[d]*tw[t3].i + f[d+1]*tw[t3].r
			s5r, s5i := f[a]-s1r, f[a+1]-s1i
			f[a] += s1r
			f[a+1] += s1i
			s3r, s3i := s0r+s2r, s0i+s2i
			s4r, s4i := s0r-s2r, s0i-s2i
			f[c], f[c+1] = f[a]-s3r, f[a+1]-s3i
			t1 += fstride
			t2 += fstride * 2
			t3 += fstride * 3
			f[a] += s3r
			f[a+1] += s3i
			f[b] = s5r + s4i
			f[b+1] = s5i - s4r
			f[d] = s5r - s4i
			f[d+1] = s5i + s4r
			k++
		}
	}
}

func kfBfly3(f []float32, fstride int, st *fftState, m, n, mm int) {
	tw := st.twiddles
	m2 := 2 * m
	epi3 := tw[fstride*m]
	for i := 0; i < n; i++ {
		k := i * mm
		t1, t2 := 0, 0
		for j := m; j > 0; j-- {
			a, b, c := 2*k, 2*(k+m), 2*(k+m2)
			s1r := f[b]*tw[t1].r - f[b+1]*tw[t1].i
			s1i := f[b]*tw[t1].i + f[b+1]*tw[t1].r
			s2r := f[c]*tw[t2].r - f[c+1]*tw[t2].i
			s2i := f[c]*tw[t2].i + f[c+1]*tw[t2].r
			s3r, s3i := s1r+s2r, s1i+s2i
			s0r, s0i := s1r-s2r, s1i-s2i
			t1 += fstride
			t2 += fstride * 2
			f[b] = f[a] - s3r*.5
			f[b+1] = f[a+1] - s3i*.5
			s0r *= epi3.i
			s0i *= epi3.i
			f[a] += s3r
			f[a+1] += s3i
			f[c] = f[b] + s0i
			f[c+1] = f[b+1] - s0r
			f[b] -= s0i
			f[b+1] += s0r
			k++
		}
	}
}

func kfBfly5(f []float32, fstride int, st *fftState, m, n, mm int) {
	tw := st.twiddles
	ya := tw[fstride*m]
	yb := tw[fstride*2*m]
	for i := 0; i < n; i++ {
		k := i * mm
		for u := 0; u < m; u++ {
			p0, p1, p2, p3, p4 := 2*(k+u), 2*(k+u+m), 2*(k+u+2*m), 2*(k+u+3*m), 2*(k+u+4*m)
			w1, w2, w3, w4 := tw[u*fstride], tw[2*u*fstride], tw[3*u*fstride], tw[4*u*fstride]
			s0r, s0i := f[p0], f[p0+1]
			s1r := f[p1]*w1.r - f[p1+1]*w1.i
			s1i := f[p1]*w1.i + f[p1+1]*w1.r
			s2r := f[p2]*w2.r - f[p2+1]*w2.i
			s2i := f[p2]*w2.i + f[p2+1]*w2.r
			s3r := f[p3]*w3.r - f[p3+1]*w3.i
			s3i := f[p3]*w3.i + f[p3+1]*w3.r
			s4r := f[p4]*w4.r - f[p4+1]*w4.i
			s4i := f[p4]*w4.i + f[p4+1]*w4.r

			s7r, s7i := s1r+s4r, s1i+s4i
			s10r, s10i := s1r-s4r, s1i-s4i
			s8r, s8i := s2r+s3r, s2i+s3i
			s9r, s9i := s2r-s3r, s2i-s3i

			f[p0] += s7r + s8r
			f[p0+1] += s7i + s8i

			s5r := s0r + (s7r*ya.r + s8r*yb.r)
			s5i := s0i + (s7i*ya.r + s8i*yb.r)
			s6r := s10i*ya.i + s9i*yb.i
			s6i := -(s10r*ya.i + s9r*yb.i)

			f[p1], f[p1+1] = s5r-s6r, s5i-s6i
			f[p4], f[p4+1] = s5r+s6r, s5i+s6i

			s11r := s0r + (s7r*yb.r + s8r*ya.r)
			s11i := s0i + (s7i*yb.r + s8i*ya.r)
			s12r := s9i*ya.i - s10i*yb.i
			s12i := s10r*yb.i - s9r*ya.i

			f[p2], f[p2+1] = s11r+s12r, s11i+s12i
			f[p3], f[p3+1] = s11r-s12r, s11i-s12i
		}
	}
}
