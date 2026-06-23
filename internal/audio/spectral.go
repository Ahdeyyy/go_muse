package audio

import "math"

// Frame parameters (at the analysis sample rate). 2048/512 at 22050 Hz gives
// ~93 ms windows hopping every ~23 ms — a standard music-analysis grid.
const (
	frameSize = 2048
	hopSize   = 512
	numMel    = 26 // mel filters
	numMFCC   = 13 // kept cepstral coefficients
)

// hannWindow returns a precomputed Hann window of length n.
func hannWindow(n int) []float64 {
	w := make([]float64, n)
	for i := range w {
		w[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(n-1)))
	}
	return w
}

// hzToMel / melToHz: standard mel scale.
func hzToMel(f float64) float64 { return 2595 * math.Log10(1+f/700) }
func melToHz(m float64) float64 { return 700 * (math.Pow(10, m/2595) - 1) }

// melFilterbank builds numMel triangular filters spanning [fmin,fmax] over the
// (frameSize/2+1) magnitude bins. Returned as filters[m][bin] weights.
func melFilterbank(sampleRate int, fmin, fmax float64) [][]float64 {
	nBins := frameSize/2 + 1
	melMin, melMax := hzToMel(fmin), hzToMel(fmax)
	// numMel+2 points => numMel triangles.
	points := make([]float64, numMel+2)
	for i := range points {
		mel := melMin + (melMax-melMin)*float64(i)/float64(numMel+1)
		points[i] = melToHz(mel)
	}
	// Convert center frequencies to fractional bin positions.
	bin := func(f float64) float64 { return f * float64(frameSize) / float64(sampleRate) }

	fb := make([][]float64, numMel)
	for m := 0; m < numMel; m++ {
		fb[m] = make([]float64, nBins)
		left, center, right := bin(points[m]), bin(points[m+1]), bin(points[m+2])
		for k := 0; k < nBins; k++ {
			fk := float64(k)
			switch {
			case fk >= left && fk <= center && center > left:
				fb[m][k] = (fk - left) / (center - left)
			case fk > center && fk <= right && right > center:
				fb[m][k] = (right - fk) / (right - center)
			}
		}
	}
	return fb
}

// dctII computes the first numMFCC type-II DCT coefficients of the log-mel
// energies — the classic MFCC step.
func dctII(logMel []float64) []float64 {
	n := len(logMel)
	out := make([]float64, numMFCC)
	for k := 0; k < numMFCC; k++ {
		var sum float64
		for i := 0; i < n; i++ {
			sum += logMel[i] * math.Cos(math.Pi*float64(k)*(float64(i)+0.5)/float64(n))
		}
		out[k] = sum
	}
	return out
}

// chromaMap precomputes, for each FFT bin, which pitch class (0..11, C=0) it
// belongs to. Bins outside the musical range map to -1 and are ignored.
func chromaMap(sampleRate int) []int {
	nBins := frameSize/2 + 1
	m := make([]int, nBins)
	for k := 0; k < nBins; k++ {
		f := float64(k) * float64(sampleRate) / float64(frameSize)
		if f < 27.5 || f > 5000 { // below A0 / above ~D8: skip
			m[k] = -1
			continue
		}
		midi := 69 + 12*math.Log2(f/440)
		pc := int(math.Round(midi)) % 12
		if pc < 0 {
			pc += 12
		}
		m[k] = pc
	}
	return m
}

// Krumhansl-Schmuckler key profiles (major/minor), used to estimate key by
// correlating with the averaged chroma vector.
var (
	majorProfile = [12]float64{6.35, 2.23, 3.48, 2.33, 4.38, 4.09, 2.52, 5.19, 2.39, 3.66, 2.29, 2.88}
	minorProfile = [12]float64{6.33, 2.68, 3.52, 5.38, 2.60, 3.53, 2.54, 4.75, 3.98, 2.69, 3.34, 3.17}
)

// estimateKey returns the best key (0..11), mode (1=major,0=minor) and a 0..1
// confidence from the averaged 12-bin chroma vector.
func estimateKey(chroma [12]float64) (key, mode int, conf float64) {
	best, bestKey, bestMode := math.Inf(-1), -1, -1
	second := math.Inf(-1)
	maj := normalizeProfile(majorProfile)
	min := normalizeProfile(minorProfile)
	c := normalizeChroma(chroma)

	consider := func(corr float64, k, md int) {
		if corr > best {
			second = best
			best, bestKey, bestMode = corr, k, md
		} else if corr > second {
			second = corr
		}
	}
	for k := 0; k < 12; k++ {
		consider(correlate(c, rotate(maj, k)), k, 1)
		consider(correlate(c, rotate(min, k)), k, 0)
	}
	if bestKey < 0 {
		return -1, -1, 0
	}
	// Confidence: gap between best and runner-up, squashed to 0..1.
	conf = math.Max(0, math.Min(1, (best-second)*2))
	return bestKey, bestMode, conf
}

func rotate(p [12]float64, k int) [12]float64 {
	var out [12]float64
	for i := 0; i < 12; i++ {
		out[i] = p[(i-k+12)%12]
	}
	return out
}

func normalizeProfile(p [12]float64) [12]float64 {
	var mean float64
	for _, v := range p {
		mean += v
	}
	mean /= 12
	var out [12]float64
	for i, v := range p {
		out[i] = v - mean
	}
	return out
}

func normalizeChroma(c [12]float64) [12]float64 {
	var mean float64
	for _, v := range c {
		mean += v
	}
	mean /= 12
	var out [12]float64
	for i, v := range c {
		out[i] = v - mean
	}
	return out
}

// correlate is the (unnormalized) Pearson numerator over the centered vectors,
// divided by the geometric norm — i.e. cosine of the centered vectors.
func correlate(a, b [12]float64) float64 {
	var num, na, nb float64
	for i := 0; i < 12; i++ {
		num += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return num / math.Sqrt(na*nb)
}
