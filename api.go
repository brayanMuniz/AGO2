package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type App struct {
	DB *sql.DB
}

// Helper to send JSON formatted error messages
func sendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// /api/image?id=X
func (a *App) handleGetImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		sendJSONError(w, "Missing 'id' path parameter", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		sendJSONError(w, "Invalid 'id' parameter; must be an integer", http.StatusBadRequest)
		return
	}

	img, err := GetImageByID(a.DB, id, true)
	if err != nil {
		// Differentiate between a 404 Not Found and a 500 Server Error
		if strings.Contains(err.Error(), "no file found") {
			sendJSONError(w, err.Error(), http.StatusNotFound)
		} else {
			sendJSONError(w, "Failed to fetch image", http.StatusInternalServerError)
			fmt.Printf("Database error fetching image %d: %v\n", id, err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(img)
}

// /api/search?tags=tag1+tag2
func (a *App) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tagsParam := r.URL.Query().Get("tags")
	if tagsParam == "" {
		sendJSONError(w, "Missing 'tags' query parameter", http.StatusBadRequest)
		return
	}

	// "?tags=tag1+tag2" becomes "tag1 tag2".
	tags := strings.Fields(tagsParam)

	images, err := SearchImagesByTags(a.DB, tags)
	if err != nil {
		sendJSONError(w, "Failed to search images", http.StatusInternalServerError)
		fmt.Printf("Database error searching tags %v: %v\n", tags, err)
		return
	}

	// If no images are found, return an empty array instead of null
	if images == nil {
		images = make([]Image, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(images)
}

// POST /api/process-gallery
func (a *App) handleProcessGallery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	apiKey := os.Getenv("DANBOORU_KEY")
	userName := os.Getenv("USERNAME")

	if apiKey == "" || userName == "" {
		fmt.Println("Error: API credentials missing from environment variables.")
		sendJSONError(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	targetDir := "./Gallery/"

	err := ProcessGalleryDirectory(a.DB, apiKey, userName, targetDir)
	if err != nil {
		fmt.Printf("Error processing gallery: %v\n", err)
		sendJSONError(w, "Failed to process gallery", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Successfully processed Gallery Directory",
	})
	fmt.Println("Finished processing POST /api/process-gallery")
}

// OPTIMIZE: Add Go routines
func ProcessGalleryDirectory(db *sql.DB, apikey, userName, dirPath string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		ext := strings.ToLower(filepath.Ext(fileName))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
			continue
		}

		filePath := filepath.Join(dirPath, fileName)

		err := ProcessNewImageUpload(db, apikey, userName, fileName, filePath)
		if err != nil {
			return err
		}

		time.Sleep(1 * time.Second) // for rate limits
	}

	return nil
}

// DELETE /api/image/{id}
func (a *App) handleDeleteImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		sendJSONError(w, "Missing 'id' parameter", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		sendJSONError(w, "Invalid 'id' parameter; must be an integer", http.StatusBadRequest)
		return
	}

	galleryDir := "./Gallery/"
	err = DeleteImageByID(a.DB, id, galleryDir)
	if err != nil {
		if strings.Contains(err.Error(), "no file found") {
			sendJSONError(w, err.Error(), http.StatusNotFound)
		} else {
			sendJSONError(w, "Failed to delete image", http.StatusInternalServerError)
			fmt.Printf("Database error deleting image %d: %v\n", id, err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Successfully deleted image ID %d", id),
	})
}

// PATCH /api/image/{id}/favorite
func (a *App) handleUpdateFavorite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch && r.Method != http.MethodPut {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		sendJSONError(w, "Missing 'id' parameter", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		sendJSONError(w, "Invalid 'id' parameter; must be an integer", http.StatusBadRequest)
		return
	}

	var reqBody struct {
		IsFavorite bool `json:"is_favorite"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		sendJSONError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	err = UpdateImageFavoriteStatus(a.DB, id, reqBody.IsFavorite)
	if err != nil {
		if strings.Contains(err.Error(), "no file found") {
			sendJSONError(w, err.Error(), http.StatusNotFound)
		} else {
			sendJSONError(w, "Failed to update favorite status", http.StatusInternalServerError)
			fmt.Printf("Database error updating favorite status for image %d: %v\n", id, err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"message":     fmt.Sprintf("Successfully updated favorite status for image ID %d", id),
		"is_favorite": reqBody.IsFavorite,
	})
}

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

	galleryDir := "./Gallery/"
	albumsBaseDir := "./Albums/"

	err := ExportImagesToAlbum(a.DB, reqBody.AlbumName, reqBody.ImageIDs, galleryDir, albumsBaseDir)
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
