package main

import (
	"crypto/sha256"
	"encoding/hex"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
)

type Image struct {
	FileName    string
	Hash        string
	MainData    *Post
	IQDBMatches []IQDBMatch
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
