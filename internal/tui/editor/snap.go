package editor

import "math"

// snapToHalf rounds v to the nearest 0.5 and clamps to [0, 24].
func snapToHalf(v float64) float64 {
	v = math.Round(v*2) / 2
	if v < 0 {
		return 0
	}
	if v > 24 {
		return 24
	}
	return v
}

// nudge adds dir * 0.5 to v and clamps to [0, 24].
// dir should be +1 or -1.
func nudge(v float64, dir int) float64 {
	v += float64(dir) * 0.5
	if v < 0 {
		return 0
	}
	if v > 24 {
		return 24
	}
	return v
}
