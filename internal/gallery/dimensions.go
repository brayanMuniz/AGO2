package gallery

import (
	"database/sql"
	"fmt"
	"image"
	"os"
)

// GetFileDimensionsAndSize inspects a physical image file to return width, height, and size.
func GetFileDimensionsAndSize(filePath string) (width int, height int, size int64) {
	fileInfo, err := os.Stat(filePath)
	if err == nil {
		size = fileInfo.Size()
	}
	f, err := os.Open(filePath)
	if err != nil {
		return 0, 0, size
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err == nil {
		width = cfg.Width
		height = cfg.Height
	}
	return width, height, size
}

// UpdateFileDimensionsInDB updates the physical file dimensions, size, hash, and colors stored in the files table.
func UpdateFileDimensionsInDB(db *sql.DB, filename, filePath, thumbnailDir string) error {
	w, h, sz := GetFileDimensionsAndSize(filePath)
	hash, err := GetPixelHash(filePath)
	if err != nil {
		return fmt.Errorf("failed to hash updated file: %w", err)
	}

	_, err = db.Exec("UPDATE files SET hash = ?, image_width = ?, image_height = ?, file_size = ? WHERE filename = ?", hash, w, h, sz, filename)
	if err != nil {
		return err
	}

	var fileID int64
	err = db.QueryRow("SELECT id FROM files WHERE filename = ?", filename).Scan(&fileID)
	if err == nil && fileID > 0 {
		palette, _ := ExtractColorPalette(filePath, 5)
		db.Exec("DELETE FROM image_colors WHERE file_id = ?", fileID)
		for _, color := range palette {
			db.Exec("INSERT INTO image_colors (file_id, r, g, b, hex, weight) VALUES (?, ?, ?, ?, ?, ?)",
				fileID, color.R, color.G, color.B, color.Hex, color.Weight)
		}
	}

	// Regenerate thumbnail so UI displays the replaced image
	GenerateThumbnail(filePath, thumbnailDir)
	return nil
}
