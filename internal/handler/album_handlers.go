package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/brayanMuniz/AGO2/internal/gallery"
)

// POST /api/album/export
func (a *App) handleExportAlbum(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqBody struct {
		AlbumName string  `json:"album_name"`
		ImageIDs  []int64 `json:"image_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		sendJSONError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(reqBody.AlbumName) == "" {
		sendJSONError(w, "Album name cannot be empty", http.StatusBadRequest)
		return
	}

	if len(reqBody.ImageIDs) == 0 {
		sendJSONError(w, "Must provide at least one image ID", http.StatusBadRequest)
		return
	}

	err := gallery.ExportImagesToAlbum(a.DB, reqBody.AlbumName, reqBody.ImageIDs, a.Cfg.GalleryDir, a.Cfg.AlbumsDir)
	if err != nil {
		sendJSONError(w, "Failed to export album", http.StatusInternalServerError)
		fmt.Printf("Error exporting album '%s': %v\n", reqBody.AlbumName, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"message":    fmt.Sprintf("Successfully exported %d images to album '%s'", len(reqBody.ImageIDs), reqBody.AlbumName),
		"album_name": reqBody.AlbumName,
	})
}
