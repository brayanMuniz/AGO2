package gallery

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

	"github.com/brayanMuniz/AGO2/internal/models"
)

// GetPixelHash decodes an image to raw RGBA pixels and returns its SHA-256 hash.
// This ignores metadata so identical images with different EXIF data produce the same hash.
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

// GenerateThumbnail creates a scaled-down JPEG thumbnail and returns its path.
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

// ExtractColorPalette extracts the top N dominant colors from an image and returns them as Color structs with weight.
// Uses saturation and vibrancy weighting so vivid character colors rank above neutral backgrounds.
func ExtractColorPalette(filePath string, numColors int) ([]models.Color, error) {
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

	type colorScore struct {
		Hex   string
		Count int
		Score float64
	}
	var scored []colorScore
	for hexStr, count := range colorCounts {
		var r, g, b int
		fmt.Sscanf(hexStr, "#%02x%02x%02x", &r, &g, &b)

		// Calculate HSL saturation and lightness
		rf := float64(r) / 255.0
		gf := float64(g) / 255.0
		bf := float64(b) / 255.0

		maxC := math.Max(rf, math.Max(gf, bf))
		minC := math.Min(rf, math.Min(gf, bf))
		lightness := (maxC + minC) / 2.0

		saturation := 0.0
		if maxC != minC {
			if lightness <= 0.5 {
				saturation = (maxC - minC) / (maxC + minC)
			} else {
				saturation = (maxC - minC) / (2.0 - maxC - minC)
			}
		}

		// Vibrancy multiplier: penalize near-black, near-white, and low-saturation (grey) colors
		vibrancy := 1.0
		if lightness < 0.08 || lightness > 0.92 {
			// Near-black or near-white: heavy penalty
			vibrancy = 0.1
		} else if lightness < 0.15 || lightness > 0.85 {
			// Very dark or very light: moderate penalty
			vibrancy = 0.3
		} else if saturation < 0.1 {
			// Desaturated greys: heavy penalty
			vibrancy = 0.15
		} else if saturation < 0.2 {
			// Low saturation: moderate penalty
			vibrancy = 0.4
		}

		// Boost mid-saturation and mid-lightness vibrant colors
		satBoost := 1.0 + saturation*1.5

		frequency := float64(count) / float64(totalPixels)
		score := frequency * vibrancy * satBoost

		scored = append(scored, colorScore{Hex: hexStr, Count: count, Score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	var palette []models.Color
	for i := 0; i < len(scored) && i < numColors; i++ {
		var r, g, b int
		fmt.Sscanf(scored[i].Hex, "#%02x%02x%02x", &r, &g, &b)

		weight := 0.0
		if totalPixels > 0 {
			weight = float64(scored[i].Count) / float64(totalPixels)
		}

		palette = append(palette, models.Color{
			R:      r,
			G:      g,
			B:      b,
			Hex:    scored[i].Hex,
			Weight: weight,
		})
	}

	return palette, nil
}
