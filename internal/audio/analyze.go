// Package audio extracts low-level DSP features from a mono waveform.
package audio

import (
	"math"

	"github.com/Ahdeyyy/go_muse/internal/decode"
	"github.com/Ahdeyyy/go_muse/internal/model"
	"gonum.org/v1/gonum/dsp/fourier"
)

const dbFloor = -90.0 // clamp for log of near-silence

// Analyze computes the full LowLevel feature set for a decoded signal.
func Analyze(sig decode.Signal) model.LowLevel {
	x := sig.Samples
	sr := sig.SampleRate
	out := model.LowLevel{
		SampleRate:  sr,
		DurationSec: sig.Duration(),
		Key:         -1,
		Mode:        -1,
		MFCC:        make([]float64, numMFCC),
	}
	if len(x) < frameSize || sr <= 0 {
		return out
	}

	// --- Global time-domain measures ---
	out.RMSDb, out.PeakDb = rmsPeakDb(x)
	out.DynamicRangeDb = out.PeakDb - out.RMSDb

	// --- Framing setup ---
	win := hannWindow(frameSize)
	fft := fourier.NewFFT(frameSize)
	nBins := frameSize/2 + 1
	freqs := make([]float64, nBins)
	for k := range freqs {
		freqs[k] = float64(k) * float64(sr) / float64(frameSize)
	}
	mel := melFilterbank(sr, 0, float64(sr)/2)
	chromaBins := chromaMap(sr)

	numFrames := 1 + (len(x)-frameSize)/hopSize

	// Accumulators over "voiced" frames (above an energy gate).
	var (
		voiced                                                  int
		sumCentroid, sumRolloff, sumBandwidth, sumFlatness      float64
		sumZCR                                                  float64
		mfccAccum                                               = make([]float64, numMFCC)
		chromaAccum                                             [12]float64
		harmonicNum, harmonicDen                                float64
		onsetEnv                                                = make([]float64, numFrames)
		prevMag                                                 = make([]float64, nBins)
		havePrev                                                bool
		frameBuf                                                = make([]float64, frameSize)
	)

	// Energy gate relative to global peak, to ignore silence between tracks.
	gate := math.Pow(10, (out.RMSDb-25)/20)

	for i := 0; i < numFrames; i++ {
		start := i * hopSize
		seg := x[start : start+frameSize]

		// Windowed copy + per-frame RMS / ZCR on the raw segment.
		var sumSq float64
		var zc int
		for j := 0; j < frameSize; j++ {
			s := float64(seg[j])
			sumSq += s * s
			if j > 0 && ((seg[j-1] >= 0) != (seg[j] >= 0)) {
				zc++
			}
			frameBuf[j] = s * win[j]
		}
		frameRMS := math.Sqrt(sumSq / float64(frameSize))

		coeffs := fft.Coefficients(nil, frameBuf)
		mag := make([]float64, nBins)
		var magSum, powSum float64
		for k := 0; k < nBins; k++ {
			m := math.Hypot(real(coeffs[k]), imag(coeffs[k]))
			mag[k] = m
			magSum += m
			powSum += m * m
		}

		// Onset detection (spectral flux) runs on every frame.
		if havePrev {
			var flux float64
			for k := 0; k < nBins; k++ {
				d := mag[k] - prevMag[k]
				if d > 0 {
					flux += d
				}
			}
			onsetEnv[i] = flux
		}
		copy(prevMag, mag)
		havePrev = true

		if frameRMS < gate || magSum == 0 {
			continue // skip near-silent frames for spectral stats
		}
		voiced++
		sumZCR += float64(zc) / float64(frameSize)

		// Spectral shape.
		var centroid, rolloffEnergy float64
		for k := 0; k < nBins; k++ {
			centroid += freqs[k] * mag[k]
		}
		centroid /= magSum
		sumCentroid += centroid

		// Rolloff: lowest freq below which 85% of magnitude lies.
		thresh := 0.85 * magSum
		var cum float64
		roll := freqs[nBins-1]
		for k := 0; k < nBins; k++ {
			cum += mag[k]
			if cum >= thresh {
				roll = freqs[k]
				break
			}
		}
		_ = rolloffEnergy
		sumRolloff += roll

		var bw float64
		for k := 0; k < nBins; k++ {
			d := freqs[k] - centroid
			bw += mag[k] * d * d
		}
		sumBandwidth += math.Sqrt(bw / magSum)

		// Flatness = geomean(power)/mean(power) over bins 1..nBins-1.
		var logSum, linSum float64
		cnt := 0
		for k := 1; k < nBins; k++ {
			p := mag[k] * mag[k]
			if p <= 0 {
				p = 1e-12
			}
			logSum += math.Log(p)
			linSum += p
			cnt++
		}
		geoMean := math.Exp(logSum / float64(cnt))
		arMean := linSum / float64(cnt)
		flat := 0.0
		if arMean > 0 {
			flat = geoMean / arMean
		}
		sumFlatness += flat
		// Harmonic ratio proxy: tonal frames (low flatness) contribute energy.
		harmonicNum += (1 - flat) * powSum
		harmonicDen += powSum

		// MFCC.
		logMel := make([]float64, numMel)
		for m := 0; m < numMel; m++ {
			var e float64
			wf := mel[m]
			for k := 0; k < nBins; k++ {
				e += wf[k] * mag[k] * mag[k]
			}
			logMel[m] = math.Log(e + 1e-10)
		}
		c := dctII(logMel)
		for j := 0; j < numMFCC; j++ {
			mfccAccum[j] += c[j]
		}

		// Chroma.
		for k := 0; k < nBins; k++ {
			pc := chromaBins[k]
			if pc >= 0 {
				chromaAccum[pc] += mag[k]
			}
		}
	}

	if voiced == 0 {
		return out
	}
	inv := 1.0 / float64(voiced)
	out.SpectralCentroid = sumCentroid * inv
	out.SpectralRolloff = sumRolloff * inv
	out.SpectralBandwidth = sumBandwidth * inv
	out.SpectralFlatness = sumFlatness * inv
	out.ZCR = sumZCR * inv
	for j := 0; j < numMFCC; j++ {
		out.MFCC[j] = mfccAccum[j] * inv
	}
	if harmonicDen > 0 {
		out.HarmonicRatio = harmonicNum / harmonicDen
	}

	// Key from averaged chroma.
	out.Key, out.Mode, out.KeyConfidence = estimateKey(chromaAccum)

	// Tempo, beat strength, onset rate from the onset envelope.
	out.TempoBPM, out.BeatStrength = estimateTempo(onsetEnv, sr)
	out.OnsetRate = onsetRate(onsetEnv, sr, out.DurationSec)

	return out
}

func rmsPeakDb(x []float32) (rmsDb, peakDb float64) {
	var sumSq, peak float64
	for _, s := range x {
		v := float64(s)
		sumSq += v * v
		a := math.Abs(v)
		if a > peak {
			peak = a
		}
	}
	rms := math.Sqrt(sumSq / float64(len(x)))
	return toDb(rms), toDb(peak)
}

func toDb(v float64) float64 {
	if v <= 0 {
		return dbFloor
	}
	d := 20 * math.Log10(v)
	if d < dbFloor {
		return dbFloor
	}
	return d
}

// estimateTempo autocorrelates the onset envelope and picks the lag whose BPM
// lies in [50,200] with the strongest periodicity.
func estimateTempo(env []float64, sr int) (bpm, strength float64) {
	n := len(env)
	if n < 8 {
		return 0, 0
	}
	// Mean-remove the envelope.
	var mean float64
	for _, v := range env {
		mean += v
	}
	mean /= float64(n)
	e := make([]float64, n)
	for i := range env {
		e[i] = env[i] - mean
	}

	frameRate := float64(sr) / float64(hopSize) // envelope samples per second
	minLag := int(frameRate * 60 / 200)         // 200 BPM
	maxLag := int(frameRate * 60 / 50)          // 50 BPM
	if maxLag >= n {
		maxLag = n - 1
	}
	if minLag < 1 {
		minLag = 1
	}

	var energy0 float64
	for _, v := range e {
		energy0 += v * v
	}
	if energy0 == 0 {
		return 0, 0
	}

	// Perceptual tempo prior: weight each lag by a log-normal bias toward ~120
	// BPM (Ellis 2007). This suppresses the common half-/double-tempo octave
	// errors that plain autocorrelation produces.
	const priorCenter, priorWidth = 120.0, 0.9
	bestLag, bestScore, bestRaw := 0, 0.0, 0.0
	for lag := minLag; lag <= maxLag; lag++ {
		var c float64
		for i := lag; i < n; i++ {
			c += e[i] * e[i-lag]
		}
		lagBPM := 60 * frameRate / float64(lag)
		w := math.Exp(-0.5 * math.Pow(math.Log2(lagBPM/priorCenter)/priorWidth, 2))
		if s := c * w; s > bestScore {
			bestScore, bestLag, bestRaw = s, lag, c
		}
	}
	if bestLag == 0 {
		return 0, 0
	}
	bpm = 60 * frameRate / float64(bestLag)
	strength = math.Max(0, math.Min(1, bestRaw/energy0))
	return bpm, strength
}

// onsetRate counts salient peaks in the onset envelope per second.
func onsetRate(env []float64, sr int, dur float64) float64 {
	n := len(env)
	if n < 3 || dur <= 0 {
		return 0
	}
	var mean, sd float64
	for _, v := range env {
		mean += v
	}
	mean /= float64(n)
	for _, v := range env {
		sd += (v - mean) * (v - mean)
	}
	sd = math.Sqrt(sd / float64(n))
	thresh := mean + 0.5*sd

	frameRate := float64(sr) / float64(hopSize)
	minGap := int(frameRate * 0.10) // refractory ~100ms
	if minGap < 1 {
		minGap = 1
	}
	count, last := 0, -minGap
	for i := 1; i < n-1; i++ {
		if env[i] > thresh && env[i] >= env[i-1] && env[i] > env[i+1] && i-last >= minGap {
			count++
			last = i
		}
	}
	return float64(count) / dur
}
