package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "golang.org/x/image/bmp"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

type Color struct {
	R      int     `json:"r"`
	G      int     `json:"g"`
	B      int     `json:"b"`
	Hex    string  `json:"hex"`
	Weight float64 `json:"weight"`
}

type Image struct {
	ID            int64  `json:"id"`
	FileName      string `json:"file_name"`
	IsFavorite    bool   `json:"is_favorite"`
	Organized     bool   `json:"organized"`
	HasDuplicate  *int64 `json:"has_duplicate,omitempty"`
	Hash          string `json:"hash"`
	MainData      *Post  `json:"main_data"`
	ThumbnailPath string `json:"thumbnail_path"`
	ImageWidth    int    `json:"image_width"`
	ImageHeight   int    `json:"image_height"`
	FileSize      int64  `json:"file_size"`
}

// RGBToLAB converts standard 8-bit sRGB (0-255) to CIE L*a*b* (D65 illuminant).
func RGBToLAB(r, g, b int) (l, a, bVal float64) {
	lin := func(c int) float64 {
		v := float64(c) / 255.0
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}

	rl := lin(r)
	gl := lin(g)
	bl := lin(b)

	x := (0.4124564*rl + 0.3575761*gl + 0.1804375*bl) / 0.95047
	y := (0.2126729*rl + 0.7151522*gl + 0.0721750*bl) / 1.00000
	z := (0.0193339*rl + 0.1191920*gl + 0.9503041*bl) / 1.08883

	f := func(t float64) float64 {
		if t > 0.008856 {
			return math.Cbrt(t)
		}
		return (7.787 * t) + (16.0 / 116.0)
	}

	fx := f(x)
	fy := f(y)
	fz := f(z)

	l = (116.0 * fy) - 16.0
	a = 500.0 * (fx - fy)
	bVal = 200.0 * (fy - fz)
	return l, a, bVal
}

// ColorDistanceLAB computes perceptual Euclidean distance Delta E in CIE L*a*b* space.
func ColorDistanceLAB(c1, c2 Color) float64 {
	l1, a1, b1 := RGBToLAB(c1.R, c1.G, c1.B)
	l2, a2, b2 := RGBToLAB(c2.R, c2.G, c2.B)
	dl := l1 - l2
	da := a1 - a2
	db := b1 - b2
	return math.Sqrt(dl*dl + da*da + db*db)
}

// ParseHexToColor converts "#RRGGBB" or "RRGGBB" into a Color struct.
func ParseHexToColor(hexStr string) Color {
	clean := strings.TrimPrefix(hexStr, "#")
	var r, g, b int
	if len(clean) == 6 {
		fmt.Sscanf(clean, "%02x%02x%02x", &r, &g, &b)
	}
	return Color{
		R:      r,
		G:      g,
		B:      b,
		Hex:    "#" + clean,
		Weight: 1.0,
	}
}

func GetPixelHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	img, _, err := image.Decode(f) // removes metadata from image
	if err != nil {
		return "", err
	}

	// Normalize image to standard RGBA
	bounds := img.Bounds()
	rgbaImg := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))

	// Draw original image onto RGBA canvas
	draw.Draw(rgbaImg, rgbaImg.Bounds(), img, bounds.Min, draw.Src)

	hasher := sha256.New()
	hasher.Write(rgbaImg.Pix)

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func GenerateThumbnail(originalPath, thumbnailDir string) (string, error) {
	file, err := os.Open(originalPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Decode the original image configurations/pixels
	srcImg, _, err := image.Decode(file)
	if err != nil {
		return "", err
	}

	bounds := srcImg.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	maxDim := 2000 // amount of pixels
	dstW, dstH := maxDim, maxDim
	if srcW > srcH {
		dstH = (srcH * maxDim) / srcW
	} else {
		dstW = (srcW * maxDim) / srcH
	}

	// Create a blank canvas for the thumbnail
	dstImg := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	white := image.NewUniform(image.White)
	draw.Draw(dstImg, dstImg.Bounds(), white, image.Point{}, draw.Src)

	// Scale the original image down to the thumbnail size over the white background
	xdraw.BiLinear.Scale(dstImg, dstImg.Bounds(), srcImg, bounds, xdraw.Over, nil)
	if err := os.MkdirAll(thumbnailDir, os.ModePerm); err != nil {
		return "", err
	}

	baseName := filepath.Base(originalPath)
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)

	thumbFilename := "thumb_" + nameWithoutExt + ".jpg"
	outPath := filepath.Join(thumbnailDir, thumbFilename)

	outFile, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	// Save with 80% JPEG quality
	err = jpeg.Encode(outFile, dstImg, &jpeg.Options{Quality: 80})
	if err != nil {
		return "", err
	}

	return outPath, nil
}

// Extracts the top N dominant colors from an image and returns them as Color structs with weight
func ExtractColorPalette(filePath string, numColors int) ([]Color, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	srcImg, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	dstImg := image.NewRGBA(image.Rect(0, 0, 50, 50))
	xdraw.BiLinear.Scale(dstImg, dstImg.Bounds(), srcImg, srcImg.Bounds(), xdraw.Over, nil)

	colorCounts := make(map[string]int)
	bounds := dstImg.Bounds()
	var totalPixels int

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := dstImg.At(x, y).RGBA()

			// Ignore transparent pixels
			if a == 0 {
				continue
			}
			totalPixels++

			// Downscale 16-bit to 8-bit, then mask the lower bits to group similar colors
			r8 := (r >> 8) & 0xF0
			g8 := (g >> 8) & 0xF0
			b8 := (b >> 8) & 0xF0

			hexStr := fmt.Sprintf("#%02x%02x%02x", r8, g8, b8)
			colorCounts[hexStr]++
		}
	}

	type colorFreq struct {
		Hex   string
		Count int
	}
	var frequencies []colorFreq
	for hexStr, count := range colorCounts {
		frequencies = append(frequencies, colorFreq{Hex: hexStr, Count: count})
	}

	sort.Slice(frequencies, func(i, j int) bool {
		return frequencies[i].Count > frequencies[j].Count
	})

	var palette []Color
	for i := 0; i < len(frequencies) && i < numColors; i++ {
		var r, g, b int
		fmt.Sscanf(frequencies[i].Hex, "#%02x%02x%02x", &r, &g, &b)

		weight := 0.0
		if totalPixels > 0 {
			weight = float64(frequencies[i].Count) / float64(totalPixels)
		}

		palette = append(palette, Color{
			R:      r,
			G:      g,
			B:      b,
			Hex:    frequencies[i].Hex,
			Weight: weight,
		})
	}

	return palette, nil
}
