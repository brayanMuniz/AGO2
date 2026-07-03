package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type App struct {
	DB *sql.DB
}

type ProcessGallerySum struct {
	Processed int `json:"processed"`
	AutoMatch int `json:"auto_match"`
	Skipped   int `json:"skipped"`
}

type JobState struct {
	sync.RWMutex
	ID         string            `json:"job_id"`
	Status     string            `json:"status"` // "processing", "completed", "failed"
	Stats      ProcessGallerySum `json:"stats"`
	TotalFiles int               `json:"total_files"`
	Error      string            `json:"error,omitempty"`
}

// WARNING: I am not currently cleaning up this job
var (
	jobStoreMu sync.RWMutex
	jobStore   = make(map[string]*JobState)
)

func generateJobID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Helper to send JSON formatted error messages
func sendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// /api/image/{id}
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

// /api/image/{id}/matches
func (a *App) handleGetImageMatches(w http.ResponseWriter, r *http.Request) {

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

	img, err := GetImageByID(a.DB, id, false)
	if err != nil {
		if strings.Contains(err.Error(), "no file found") {
			sendJSONError(w, err.Error(), http.StatusNotFound)
		} else {
			sendJSONError(w, "Failed to fetch image", http.StatusInternalServerError)
			fmt.Printf("Database error fetching image %d: %v\n", id, err)
		}
		return
	}

	apiKey := os.Getenv("DANBOORU_KEY")
	userName := os.Getenv("USERNAME")
	fileLocation := "./Gallery/" + img.FileName

	matches, err := iqdb_upload_request(apiKey, userName, fileLocation)
	if err != nil {
		sendJSONError(w, "Failed to fetch image data", http.StatusInternalServerError)
		fmt.Printf("Could not get iqdb request for %d", id)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(matches)
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
	targetDir := "./Gallery/"

	job := &JobState{
		ID:     generateJobID(),
		Status: "processing",
		Stats:  ProcessGallerySum{Processed: 0, AutoMatch: 0, Skipped: 0},
	}

	// save job
	jobStoreMu.Lock()
	jobStore[job.ID] = job
	jobStoreMu.Unlock()

	go runGalleryWorker(a.DB, apiKey, userName, targetDir, job)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted) // 202 Accepted is standard for "started processing"
	json.NewEncoder(w).Encode(map[string]string{
		"job_id":  job.ID,
		"message": "Processing started in the background",
	})
}

// GET /api/process-gallery/status?job_id=xyz
func (a *App) handleGetJobStatus(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		sendJSONError(w, "job_id is required", http.StatusBadRequest)
		return
	}

	jobStoreMu.RLock()
	job, exists := jobStore[jobID]
	jobStoreMu.RUnlock()

	if !exists {
		sendJSONError(w, "Job not found", http.StatusNotFound)
		return
	}

	job.RLock()
	defer job.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func runGalleryWorker(db *sql.DB, apikey, userName, dirPath string, job *JobState) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		job.Lock()
		job.Status = "failed"
		job.Error = err.Error()
		job.Unlock()
		return
	}

	// Only count images in ./Gallery
	validEntries := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
			validEntries++
		}
	}

	job.Lock()
	job.TotalFiles = validEntries
	job.Unlock()

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

		r, err := ProcessNewImageUpload(db, apikey, userName, fileName, filePath)
		if err != nil {
			fmt.Printf("Error processing %s: %v\n", fileName, err)
			continue
		}

		job.Lock()
		if r.Skipped {
			job.Stats.Skipped++
		} else {
			job.Stats.Processed++
			if r.AutoMatch {
				job.Stats.AutoMatch++
			}
		}
		job.Unlock()

		// If not skipped and made apiCall, rate limit
		if !r.Skipped {
			time.Sleep(1 * time.Second)
		}
	}

	job.Lock()
	job.Status = "completed"
	job.Unlock()
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

// PATCH /api/image/{id}
func (a *App) handleImageUpdate(w http.ResponseWriter, r *http.Request) {
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
		IsFavorite       *bool  `json:"is_favorite,omitempty"`
		ActiveMetadataID *int64 `json:"active_metadata_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		sendJSONError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	params := UpdateImageParams{
		IsFavorite:       reqBody.IsFavorite,
		ActiveMetadataID: reqBody.ActiveMetadataID,
	}

	err = UpdateImage(a.DB, id, params)
	if err != nil {
		if strings.Contains(err.Error(), "no file found") {
			sendJSONError(w, err.Error(), http.StatusNotFound)
		} else {
			sendJSONError(w, "Failed to update image", http.StatusInternalServerError)
			fmt.Printf("Database error updating image %d: %v\n", id, err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"message": fmt.Sprintf("Successfully updated image ID %d", id),
	})

}

// GET /api/images/unmatched
func (a *App) handleGetUnmatchedImages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	images, err := GetImagesWithoutMetadata(a.DB)
	if err != nil {
		sendJSONError(w, "Failed to fetch unmatched images", http.StatusInternalServerError)
		fmt.Printf("Database error fetching unmatched images: %v\n", err)
		return
	}

	if images == nil {
		images = make([]Image, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(images)
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

func ProxyImageHandler(w http.ResponseWriter, r *http.Request) {
	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		http.Error(w, "Missing URL parameter", http.StatusBadRequest)
		return
	}

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	// Danbooru requires a User-Agent, otherwise they block the request
	req.Header.Set("User-Agent", "MyGoApp/1.0")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to fetch image", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Pass the content type (e.g., image/jpeg) and the image bytes back to the frontend
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
