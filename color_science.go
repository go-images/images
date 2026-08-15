package images

import (
	"fmt"
	"image"
	"math"
)

// This file implements the CIE colour-science conversions that scikit-image
// exposes in its skimage.color module: sRGB <-> CIE XYZ, sRGB <-> CIELAB, the
// sRGB companding (gamma) transfer functions, a power-law gamma adjustment
// (skimage.exposure.adjust_gamma), and the CIE76 colour difference. The
// numerical constants — the RGB<->XYZ matrices and the D65 reference white — are
// exactly those scikit-image uses, so the results match it to floating-point
// precision (and byte-for-byte after the same round-half-to-even quantisation).

// ChannelImage is a dense H-by-W image of three float64 channels stored
// interleaved (three values per pixel, row-major). It carries the colour spaces
// that do not fit in 8-bit RGBA — CIE XYZ and CIELAB — mirroring scikit-image's
// H×W×3 float arrays. It has no alpha channel: like scikit-image, the colour
// conversions operate on the three colour channels only.
type ChannelImage struct {
	// Pix holds W*H*3 values, three consecutive per pixel.
	Pix []float64
	// W and H are the image dimensions in pixels.
	W, H int
}

// NewChannelImage allocates a zeroed w-by-h three-channel float image.
func NewChannelImage(w, h int) *ChannelImage {
	return &ChannelImage{Pix: make([]float64, 3*w*h), W: w, H: h}
}

// xyzFromRGB converts linear-light sRGB to CIE XYZ (D65). These are exactly the
// coefficients scikit-image stores in colorconv.xyz_from_rgb.
var xyzFromRGB = [3][3]float64{
	{0.412453, 0.357580, 0.180423},
	{0.212671, 0.715160, 0.072169},
	{0.019334, 0.119193, 0.950227},
}

// rgbFromXYZ is the inverse of xyzFromRGB, hardcoded to the exact values
// scikit-image obtains from numpy.linalg.inv (colorconv.rgb_from_xyz) so that
// XYZ->RGB matches it in the last bits rather than reproducing the inverse with
// a possibly different rounding.
var rgbFromXYZ = [3][3]float64{
	{3.2404813432005266, -1.5371515162713183, -0.4985363261688878},
	{-0.9692549499965682, 1.8759900014898907, 0.04155592655829283},
	{0.05564663913517717, -0.20404133836651123, 1.0573110696453443},
}

// d65White is the CIE D65 reference white (2-degree observer) that scikit-image
// normalises by in xyz2lab / lab2xyz (colorconv, illuminant "D65", observer
// "2"). Note it differs very slightly from the column sums of xyzFromRGB, which
// is why converting pure white to Lab yields L*=100 with a and b a few
// thousandths off zero — scikit-image has the exact same tiny offset.
var d65White = [3]float64{0.95047, 1.0, 1.08883}

// SRGBToLinear converts one sRGB channel value in [0, 1] to linear-light
// intensity using the IEC 61966-2-1 inverse companding (the same transfer
// function scikit-image applies inside rgb2xyz).
func SRGBToLinear(c float64) float64 {
	if c > 0.04045 {
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return c / 12.92
}

// LinearToSRGB is the inverse of [SRGBToLinear]: it applies the sRGB forward
// companding to a linear channel. Values at or below 0.0031308 (including
// negatives produced by an out-of-gamut XYZ colour) take the linear branch, so
// no fractional power of a negative number is ever taken — matching
// scikit-image's masked companding in xyz2rgb.
func LinearToSRGB(c float64) float64 {
	if c > 0.0031308 {
		return 1.055*math.Pow(c, 1.0/2.4) - 0.055
	}
	return 12.92 * c
}

// RGBToXYZ converts img to the CIE XYZ colour space (D65), returning a
// three-channel float image. It reproduces skimage.color.rgb2xyz: each channel
// is scaled to [0, 1], inverse-companded to linear light, then multiplied by the
// sRGB->XYZ matrix. Alpha is ignored.
func RGBToXYZ(img image.Image) *ChannelImage {
	src := ToRGBA(img)
	b := src.Bounds()
	out := NewChannelImage(b.Dx(), b.Dy())
	for i, p := 0, 0; i < len(src.Pix); i, p = i+4, p+3 {
		r := SRGBToLinear(float64(src.Pix[i]) / 255)
		g := SRGBToLinear(float64(src.Pix[i+1]) / 255)
		bl := SRGBToLinear(float64(src.Pix[i+2]) / 255)
		out.Pix[p] = xyzFromRGB[0][0]*r + xyzFromRGB[0][1]*g + xyzFromRGB[0][2]*bl
		out.Pix[p+1] = xyzFromRGB[1][0]*r + xyzFromRGB[1][1]*g + xyzFromRGB[1][2]*bl
		out.Pix[p+2] = xyzFromRGB[2][0]*r + xyzFromRGB[2][1]*g + xyzFromRGB[2][2]*bl
	}
	return out
}

// XYZToRGB converts a CIE XYZ image back to an 8-bit sRGB image, reproducing
// skimage.color.xyz2rgb followed by scikit-image's ubyte conversion: the inverse
// matrix, forward companding, a clamp to [0, 1], then a round-half-to-even
// quantisation to bytes. The returned image is fully opaque (alpha 255).
func XYZToRGB(c *ChannelImage) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, c.W, c.H))
	for p, i := 0, 0; p < len(c.Pix); p, i = p+3, i+4 {
		x, y, z := c.Pix[p], c.Pix[p+1], c.Pix[p+2]
		r := LinearToSRGB(rgbFromXYZ[0][0]*x + rgbFromXYZ[0][1]*y + rgbFromXYZ[0][2]*z)
		g := LinearToSRGB(rgbFromXYZ[1][0]*x + rgbFromXYZ[1][1]*y + rgbFromXYZ[1][2]*z)
		bl := LinearToSRGB(rgbFromXYZ[2][0]*x + rgbFromXYZ[2][1]*y + rgbFromXYZ[2][2]*z)
		dst.Pix[i] = unitToByte(r)
		dst.Pix[i+1] = unitToByte(g)
		dst.Pix[i+2] = unitToByte(bl)
		dst.Pix[i+3] = 255
	}
	return dst
}

// RGBToLab converts img to CIELAB (D65), returning a three-channel float image
// of L*, a*, b*. It reproduces skimage.color.rgb2lab (rgb2xyz composed with
// xyz2lab). L* is in [0, 100]; a* and b* are unbounded but typically in about
// [-128, 128]. Alpha is ignored.
func RGBToLab(img image.Image) *ChannelImage {
	xyz := RGBToXYZ(img)
	for p := 0; p < len(xyz.Pix); p += 3 {
		fx := labF(xyz.Pix[p] / d65White[0])
		fy := labF(xyz.Pix[p+1] / d65White[1])
		fz := labF(xyz.Pix[p+2] / d65White[2])
		xyz.Pix[p] = 116*fy - 16
		xyz.Pix[p+1] = 500 * (fx - fy)
		xyz.Pix[p+2] = 200 * (fy - fz)
	}
	return xyz
}

// LabToRGB converts a CIELAB image back to an 8-bit sRGB image, reproducing
// skimage.color.lab2rgb (lab2xyz composed with xyz2rgb) followed by the
// round-half-to-even ubyte quantisation. The returned image is fully opaque.
func LabToRGB(c *ChannelImage) *image.RGBA {
	xyz := NewChannelImage(c.W, c.H)
	for p := 0; p < len(c.Pix); p += 3 {
		l, a, bb := c.Pix[p], c.Pix[p+1], c.Pix[p+2]
		fy := (l + 16) / 116
		fx := a/500 + fy
		fz := fy - bb/200
		if fz < 0 { // scikit-image clips a negative Z in f-space, before the
			fz = 0 // inverse nonlinearity, not after it.
		}
		xyz.Pix[p] = labFInv(fx) * d65White[0]
		xyz.Pix[p+1] = labFInv(fy) * d65White[1]
		xyz.Pix[p+2] = labFInv(fz) * d65White[2]
	}
	return XYZToRGB(xyz)
}

// labF is the CIELAB forward nonlinearity used by xyz2lab: the cube root above
// the 0.008856 threshold, and the linear tail below it. It matches
// scikit-image's masked expression.
func labF(t float64) float64 {
	if t > 0.008856 {
		return math.Cbrt(t)
	}
	return 7.787*t + 16.0/116.0
}

// labFInv inverts labF, using scikit-image's exact threshold constant
// (0.2068966, the cube root of 0.008856 rounded as scikit-image stores it).
func labFInv(t float64) float64 {
	if t > 0.2068966 {
		return t * t * t
	}
	return (t - 16.0/116.0) / 7.787
}

// AdjustGamma applies a power-law gamma correction, reproducing
// skimage.exposure.adjust_gamma with the default gain of 1: each 8-bit channel
// value v becomes round((v/255)**gamma * 255), via a 256-entry lookup table.
// Alpha is preserved unchanged. It returns an error for a negative gamma, which
// scikit-image rejects as well.
func AdjustGamma(img image.Image, gamma float64) (*image.RGBA, error) {
	if gamma < 0 {
		return nil, fmt.Errorf("images: adjust gamma: gamma must be non-negative, got %g", gamma)
	}
	// With the default gain of 1 and gamma >= 0, (i/255)**gamma lies in [0, 1],
	// so the scaled value is always in [0, 255] and needs no clamp — the byte
	// LUT is exact. This matches skimage.exposure.adjust_gamma's uint8 path
	// (255 * (linspace(0,1,256) ** gamma), rounded to nearest even).
	var lut [256]uint8
	for i := range lut {
		lut[i] = uint8(math.RoundToEven(math.Pow(float64(i)/255, gamma) * 255))
	}
	src := ToRGBA(img)
	dst := newLike(src)
	for i := 0; i < len(src.Pix); i += 4 {
		dst.Pix[i] = lut[src.Pix[i]]
		dst.Pix[i+1] = lut[src.Pix[i+1]]
		dst.Pix[i+2] = lut[src.Pix[i+2]]
		dst.Pix[i+3] = src.Pix[i+3]
	}
	return dst, nil
}

// DeltaE76 returns the per-pixel CIE76 colour difference (the Euclidean distance
// in CIELAB) between two Lab images of equal size, matching
// skimage.color.deltaE_cie76. The result has one value per pixel, row-major. It
// returns an error if the two images differ in size.
func DeltaE76(a, b *ChannelImage) ([]float64, error) {
	if a.W != b.W || a.H != b.H {
		return nil, fmt.Errorf("images: deltaE76: size mismatch %dx%d vs %dx%d", a.W, a.H, b.W, b.H)
	}
	out := make([]float64, a.W*a.H)
	for i, p := 0, 0; p < len(a.Pix); i, p = i+1, p+3 {
		dl := a.Pix[p] - b.Pix[p]
		da := a.Pix[p+1] - b.Pix[p+1]
		db := a.Pix[p+2] - b.Pix[p+2]
		out[i] = math.Sqrt(dl*dl + da*da + db*db)
	}
	return out, nil
}

// unitToByte clamps a channel value to [0, 1] and quantises it to a byte with
// round-half-to-even, matching scikit-image's img_as_ubyte (numpy.rint).
func unitToByte(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(math.RoundToEven(v * 255))
}
