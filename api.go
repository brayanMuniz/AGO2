package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

	userName, apiKey := GetDanbooruCredentials(a.DB)
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

	userName, apiKey := GetDanbooruCredentials(a.DB)
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

		info, infoErr := entry.Info()
		if infoErr == nil {
			var existingOrganized bool
			var existingFileSize int64
			err := db.QueryRow("SELECT organized, file_size FROM files WHERE filename = ?", fileName).Scan(&existingOrganized, &existingFileSize)
			if err == nil && existingOrganized && existingFileSize == info.Size() && existingFileSize > 0 {
				job.Lock()
				job.Stats.Skipped++
				job.Unlock()
				continue
			}
		}

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

// POST /api/image/batch-delete
func (a *App) handleBatchDeleteImages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		IDs []int64 `json:"ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if len(req.IDs) == 0 {
		sendJSONError(w, "No IDs provided", http.StatusBadRequest)
		return
	}

	galleryDir := "./Gallery/"
	var failedIDs []int64

	// Loop through and delete
	for _, id := range req.IDs {
		err := DeleteImageByID(a.DB, id, galleryDir)
		if err != nil {
			fmt.Printf("Failed to delete image %d: %v\n", id, err)
			failedIDs = append(failedIDs, id)
			// We continue to try deleting the rest even if one fails
		}
	}

	w.Header().Set("Content-Type", "application/json")

	if len(failedIDs) > 0 {
		// Partial failure response
		w.WriteHeader(http.StatusMultiStatus)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":  fmt.Sprintf("Deleted with some errors. %d failed.", len(failedIDs)),
			"failures": failedIDs,
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Successfully deleted %d images", len(req.IDs)),
	})
}

func DownloadAndReplaceImage(db *sql.DB, urlStr, destPath string) error {
	client := &http.Client{Timeout: 45 * time.Second}

	userName, apiKey := GetDanbooruCredentials(db)

	attemptDownload := func(targetURL string, userAgent string) (*http.Response, error) {
		req, err := http.NewRequest("GET", targetURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "image/*,*/*;q=0.8")
		return client.Do(req)
	}

	userAgent := "AGO2-GalleryOrganizer/1.0"
	if userName != "" {
		userAgent = fmt.Sprintf("AGO2-GalleryOrganizer/1.0 (by %s on Danbooru)", userName)
	}

	resp, err := attemptDownload(urlStr, userAgent)
	if err != nil {
		return fmt.Errorf("failed to download image from %s: %w", urlStr, err)
	}

	// If 403 Forbidden or 401 Unauthorized, try attaching Danbooru API credentials
	if (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized) && userName != "" && apiKey != "" {
		resp.Body.Close()
		u, parseErr := url.Parse(urlStr)
		if parseErr == nil {
			q := u.Query()
			q.Set("login", userName)
			q.Set("api_key", apiKey)
			u.RawQuery = q.Encode()
			resp, err = attemptDownload(u.String(), userAgent)
			if err != nil {
				return fmt.Errorf("retry with auth failed: %w", err)
			}
		}
	}

	// If still failing with 403, try fallback User-Agent matching iqdb.go
	if resp.StatusCode == http.StatusForbidden {
		resp.Body.Close()
		resp, err = attemptDownload(urlStr, "MyGoApp/1.0")
		if err != nil {
			return fmt.Errorf("retry with fallback user-agent failed: %w", err)
		}
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status fetching image (%d %s) from %s", resp.StatusCode, resp.Status, urlStr)
	}

	tmpPath := destPath + ".tmp_download"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	written, err := io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to save image data: %w", err)
	}
	if written == 0 {
		os.Remove(tmpPath)
		return fmt.Errorf("downloaded file was empty (0 bytes)")
	}

	return os.Rename(tmpPath, destPath)
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
		sendJSONError(w, "Invalid 'id' parameter", http.StatusBadRequest)
		return
	}

	var reqBody struct {
		IsFavorite       *bool  `json:"is_favorite,omitempty"`
		ActiveMetadataID *int64 `json:"active_metadata_id,omitempty"`
		MainData         *Post  `json:"main_data,omitempty"`
		ReplaceImage     *bool  `json:"replace_image,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		sendJSONError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	params := UpdateImageParams{
		IsFavorite:       reqBody.IsFavorite,
		ActiveMetadataID: reqBody.ActiveMetadataID,
		MainData:         reqBody.MainData,
		ReplaceImage:     reqBody.ReplaceImage,
	}

	var filename string
	err = a.DB.QueryRow("SELECT filename FROM files WHERE id = ?", id).Scan(&filename)
	if err != nil {
		sendJSONError(w, "Image not found", http.StatusNotFound)
		return
	}

	err = UpdateImage(a.DB, id, params)
	if err != nil {
		sendJSONError(w, "Failed to update image metadata", http.StatusInternalServerError)
		fmt.Printf("Database error updating image %d: %v\n", id, err)
		return
	}

	if params.ReplaceImage != nil && *params.ReplaceImage && params.MainData != nil {
		targetURL := params.MainData.FileURL
		if targetURL == "" {
			targetURL = params.MainData.LargeFileURL // funny enough this is smaller than regular FileURL
		}

		if targetURL != "" {
			destPath := filepath.Join("./Gallery/", filename)
			err := DownloadAndReplaceImage(a.DB, targetURL, destPath)
			if err != nil {
				fmt.Printf("Error replacing physical file %s: %v\n", filename, err)
				sendJSONError(w, fmt.Sprintf("Failed to download and replace file: %v", err), http.StatusInternalServerError)
				return
			}
			fmt.Printf("Successfully replaced physical file for: %s\n", filename)
			dimErr := UpdateFileDimensionsInDB(a.DB, filename, destPath)
			if dimErr != nil {
				fmt.Printf("Warning: failed to update file dimensions in DB for %s: %v\n", filename, dimErr)
			} else {
				fmt.Printf("Successfully updated file dimensions in DB for: %s\n", filename)
			}
		} else {
			// TODO: If the user tries to replace an image, but it does not let you we hit this
			fmt.Println("Bro theres not target_url")
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"message": fmt.Sprintf("Successfully updated image ID %d", id),
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

type TagSuggestion struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Count    int    `json:"count,omitempty"`
}

var (
	tagToCategoryList []TagSuggestion
	tagToCategoryOnce sync.Once
)

func loadTagToCategoryJSON() {
	tagToCategoryOnce.Do(func() {
		fileBytes, err := os.ReadFile("./ui/tag_to_category.json")
		if err != nil {
			fmt.Printf("Warning: failed to read ./ui/tag_to_category.json: %v\n", err)
			return
		}
		var rawMap map[string]string
		if err := json.Unmarshal(fileBytes, &rawMap); err != nil {
			fmt.Printf("Warning: failed to unmarshal ./ui/tag_to_category.json: %v\n", err)
			return
		}
		for k, v := range rawMap {
			tagToCategoryList = append(tagToCategoryList, TagSuggestion{
				Name:     k,
				Category: v,
			})
		}
		fmt.Printf("Loaded %d tags from ./ui/tag_to_category.json\n", len(tagToCategoryList))
	})
}

// GET /api/tags/autocomplete?query=...&category=...
func (a *App) handleTagAutocomplete(w http.ResponseWriter, r *http.Request) {
	loadTagToCategoryJSON()

	queryStr := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query")))
	categoryFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("category")))

	// Check if query starts with a category prefix (e.g. "artist:miku")
	if colonIdx := strings.Index(queryStr, ":"); colonIdx > 0 && categoryFilter == "" {
		prefix := queryStr[:colonIdx]
		catMap := map[string]string{
			"artist":    "artist",
			"character": "character",
			"copyright": "copyright",
			"general":   "general",
			"meta":      "meta",
			"rating":    "rating",
			"year":      "year",
		}
		if cat, ok := catMap[prefix]; ok {
			categoryFilter = cat
			queryStr = strings.TrimSpace(queryStr[colonIdx+1:])
		}
	}

	prefixMatches := []TagSuggestion{}
	containsMatches := []TagSuggestion{}

	for _, item := range tagToCategoryList {
		if categoryFilter != "" && !strings.EqualFold(item.Category, categoryFilter) {
			continue
		}
		if queryStr == "" {
			prefixMatches = append(prefixMatches, item)
			if len(prefixMatches) >= 30 {
				break
			}
			continue
		}

		lowerName := strings.ToLower(item.Name)
		if strings.HasPrefix(lowerName, queryStr) {
			prefixMatches = append(prefixMatches, item)
		} else if strings.Contains(lowerName, queryStr) {
			containsMatches = append(containsMatches, item)
		}
	}

	suggestions := append(prefixMatches, containsMatches...)

	// DB fallback: find tags in the database that weren't in the JSON file
	if queryStr != "" && len(suggestions) < 30 {
		// Build a set of already-found tag names for deduplication
		foundNames := make(map[string]bool)
		for _, s := range suggestions {
			foundNames[strings.ToLower(s.Name)] = true
		}

		dbQuery := `SELECT name, category FROM tags WHERE LOWER(name) LIKE ?`
		dbArgs := []interface{}{"%" + queryStr + "%"}

		if categoryFilter != "" {
			dbQuery += ` AND LOWER(category) = ?`
			dbArgs = append(dbArgs, categoryFilter)
		}

		dbQuery += ` ORDER BY name LIMIT ?`
		remaining := 30 - len(suggestions)
		dbArgs = append(dbArgs, remaining+10) // fetch a few extra to account for dedup filtering

		rows, err := a.DB.Query(dbQuery, dbArgs...)
		if err == nil {
			defer rows.Close()
			for rows.Next() && len(suggestions) < 30 {
				var name, category string
				if err := rows.Scan(&name, &category); err != nil {
					continue
				}
				if foundNames[strings.ToLower(name)] {
					continue
				}
				foundNames[strings.ToLower(name)] = true
				suggestions = append(suggestions, TagSuggestion{
					Name:     name,
					Category: category,
				})
			}
		}
	}

	if len(suggestions) > 30 {
		suggestions = suggestions[:30]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(suggestions)
}

// GET /api/filters
func (a *App) handleGetSavedFilters(w http.ResponseWriter, r *http.Request) {
	filters, err := GetAllSavedFilters(a.DB)
	if err != nil {
		sendJSONError(w, "Failed to fetch saved filters", http.StatusInternalServerError)
		fmt.Printf("Error fetching saved filters: %v\n", err)
		return
	}

	if filters == nil {
		filters = make([]SavedFilter, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filters)
}

// POST /api/filters
func (a *App) handleCreateSavedFilter(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		Name      string `json:"name"`
		Query     string `json:"query"`
		SortBy    string `json:"sort_by"`
		SortOrder string `json:"sort_order"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		sendJSONError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(reqBody.Name) == "" {
		sendJSONError(w, "Filter name cannot be empty", http.StatusBadRequest)
		return
	}

	filter, err := CreateSavedFilter(a.DB, strings.TrimSpace(reqBody.Name), reqBody.Query, reqBody.SortBy, reqBody.SortOrder)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			sendJSONError(w, fmt.Sprintf("A filter named '%s' already exists", reqBody.Name), http.StatusConflict)
		} else {
			sendJSONError(w, "Failed to create saved filter", http.StatusInternalServerError)
			fmt.Printf("Error creating saved filter: %v\n", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(filter)
}

// PUT /api/filters/{id}
func (a *App) handleUpdateSavedFilter(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		sendJSONError(w, "Invalid filter ID", http.StatusBadRequest)
		return
	}

	var reqBody struct {
		Name      string `json:"name"`
		Query     string `json:"query"`
		SortBy    string `json:"sort_by"`
		SortOrder string `json:"sort_order"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		sendJSONError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(reqBody.Name) == "" {
		sendJSONError(w, "Filter name cannot be empty", http.StatusBadRequest)
		return
	}

	filter, err := UpdateSavedFilter(a.DB, id, strings.TrimSpace(reqBody.Name), reqBody.Query, reqBody.SortBy, reqBody.SortOrder)
	if err != nil {
		sendJSONError(w, "Failed to update saved filter", http.StatusInternalServerError)
		fmt.Printf("Error updating saved filter %d: %v\n", id, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filter)
}

// DELETE /api/filters/{id}
func (a *App) handleDeleteSavedFilter(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		sendJSONError(w, "Invalid filter ID", http.StatusBadRequest)
		return
	}

	if err := DeleteSavedFilter(a.DB, id); err != nil {
		if strings.Contains(err.Error(), "no saved filter found") {
			sendJSONError(w, err.Error(), http.StatusNotFound)
		} else {
			sendJSONError(w, "Failed to delete saved filter", http.StatusInternalServerError)
			fmt.Printf("Error deleting saved filter %d: %v\n", id, err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Deleted saved filter ID %d", id),
	})
}

// GET /api/palettes
func (a *App) handleGetSavedPalettes(w http.ResponseWriter, r *http.Request) {
	palettes, err := GetSavedPalettes(a.DB)
	if err != nil {
		sendJSONError(w, "Failed to retrieve saved palettes", http.StatusInternalServerError)
		return
	}
	if palettes == nil {
		palettes = make([]SavedPalette, 0)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(palettes)
}

// POST /api/palettes
func (a *App) handleCreateSavedPalette(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		Name   string   `json:"name"`
		Colors []string `json:"colors"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		sendJSONError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(reqBody.Name) == "" {
		sendJSONError(w, "Palette name cannot be empty", http.StatusBadRequest)
		return
	}
	if len(reqBody.Colors) == 0 {
		sendJSONError(w, "Palette must contain at least one color", http.StatusBadRequest)
		return
	}

	palette, err := CreateSavedPalette(a.DB, strings.TrimSpace(reqBody.Name), reqBody.Colors)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			sendJSONError(w, fmt.Sprintf("A palette named '%s' already exists", reqBody.Name), http.StatusConflict)
		} else {
			sendJSONError(w, "Failed to save palette", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(palette)
}

// DELETE /api/palettes/{id}
func (a *App) handleDeleteSavedPalette(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		sendJSONError(w, "Invalid palette ID", http.StatusBadRequest)
		return
	}

	if err := DeleteSavedPalette(a.DB, id); err != nil {
		sendJSONError(w, "Failed to delete saved palette", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Deleted saved palette ID %d", id),
	})
}

// POST /api/palettes/extract
func (a *App) handleExtractPaletteFromImages(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil || len(reqBody.IDs) == 0 {
		sendJSONError(w, "Invalid or empty IDs list", http.StatusBadRequest)
		return
	}

	colors, err := ExtractCombinedPaletteFromImages(a.DB, reqBody.IDs)
	if err != nil {
		sendJSONError(w, "Failed to extract palette from selected images", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{
		"colors": colors,
	})
}

// GET /api/stats
func (a *App) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse optional query params
	tagLimit := 0
	if v := r.URL.Query().Get("tag_limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			tagLimit = parsed
		}
	}

	predictiveLimit := 0
	if v := r.URL.Query().Get("predictive_limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			predictiveLimit = parsed
		}
	}

	minCount := 5
	if v := r.URL.Query().Get("min_count"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			minCount = parsed
		}
	}

	artistLimit := 15
	if v := r.URL.Query().Get("artist_limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			artistLimit = parsed
		}
	}

	// Gather all stats
	library, err := GetLibraryStats(a.DB)
	if err != nil {
		sendJSONError(w, "Failed to get library stats", http.StatusInternalServerError)
		fmt.Printf("Error getting library stats: %v\n", err)
		return
	}

	categories := []string{"artist", "character", "copyright", "general"}
	leaderboards := make(map[string][]TagLeaderboardEntry)
	leaderboardsFav := make(map[string][]TagLeaderboardEntry)
	for _, cat := range categories {
		entries, err := GetTagLeaderboard(a.DB, cat, tagLimit)
		if err != nil {
			sendJSONError(w, "Failed to get tag leaderboard", http.StatusInternalServerError)
			fmt.Printf("Error getting %s leaderboard: %v\n", cat, err)
			return
		}
		leaderboards[cat] = entries

		favEntries, err := GetTagLeaderboardByFavorites(a.DB, cat, tagLimit)
		if err != nil {
			sendJSONError(w, "Failed to get favorites tag leaderboard", http.StatusInternalServerError)
			fmt.Printf("Error getting %s favorites leaderboard: %v\n", cat, err)
			return
		}
		leaderboardsFav[cat] = favEntries
	}

	ratingDist, err := GetRatingDistribution(a.DB)
	if err != nil {
		sendJSONError(w, "Failed to get rating distribution", http.StatusInternalServerError)
		fmt.Printf("Error getting rating distribution: %v\n", err)
		return
	}

	predictiveByRating, err := GetPredictiveTagAnalytics(a.DB, minCount, predictiveLimit)
	if err != nil {
		sendJSONError(w, "Failed to get predictive analytics", http.StatusInternalServerError)
		fmt.Printf("Error getting predictive analytics: %v\n", err)
		return
	}

	artistProfiles, err := GetArtistProfiles(a.DB, artistLimit)
	if err != nil {
		sendJSONError(w, "Failed to get artist profiles", http.StatusInternalServerError)
		fmt.Printf("Error getting artist profiles: %v\n", err)
		return
	}

	payload := StatsPayload{
		Library:            library,
		TagLeaderboards:    leaderboards,
		TagLeaderboardsFav: leaderboardsFav,
		RatingDist:         ratingDist,
		PredictiveByRating: predictiveByRating,
		ArtistProfiles:     artistProfiles,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

// POST /api/image/download-match
func (a *App) handleDownloadMatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqBody struct {
		Post *Post `json:"post"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil || reqBody.Post == nil {
		sendJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	targetURL := reqBody.Post.FileURL
	if targetURL == "" {
		targetURL = reqBody.Post.LargeFileURL
	}
	if targetURL == "" {
		sendJSONError(w, "Danbooru post has no downloadable file URL", http.StatusBadRequest)
		return
	}

	u, err := url.Parse(targetURL)
	ext := ".jpg"
	if err == nil {
		ext = filepath.Ext(u.Path)
		if ext == "" {
			ext = ".jpg"
		}
	}

	filename := fmt.Sprintf("danbooru_%d%s", reqBody.Post.ID, ext)
	destPath := filepath.Join("./Gallery", filename)

	counter := 1
	for {
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			break
		}
		filename = fmt.Sprintf("danbooru_%d_%d%s", reqBody.Post.ID, counter, ext)
		destPath = filepath.Join("./Gallery", filename)
		counter++
	}

	if err := DownloadAndReplaceImage(a.DB, targetURL, destPath); err != nil {
		sendJSONError(w, fmt.Sprintf("Failed to download image: %v", err), http.StatusInternalServerError)
		return
	}

	hash, err := GetPixelHash(destPath)
	if err != nil {
		os.Remove(destPath)
		sendJSONError(w, "Failed to hash downloaded image", http.StatusInternalServerError)
		return
	}
	imgWidth, imgHeight, fileSize := GetFileDimensionsAndSize(destPath)

	execRes, err := a.DB.Exec("INSERT INTO files (filename, hash, isFavorite, organized, image_width, image_height, file_size) VALUES (?, ?, FALSE, TRUE, ?, ?, ?)", filename, hash, imgWidth, imgHeight, fileSize)
	if err != nil {
		os.Remove(destPath)
		sendJSONError(w, "Failed to save file record to DB", http.StatusInternalServerError)
		return
	}
	newFileID, _ := execRes.LastInsertId()

	thumbPath, thumbErr := GenerateThumbnail(destPath, "thumbnails")
	if thumbErr == nil {
		a.DB.Exec("UPDATE files SET thumbnail_path = ? WHERE id = ?", thumbPath, newFileID)
	}

	palette, _ := ExtractColorPalette(destPath, 5)
	for _, color := range palette {
		a.DB.Exec("INSERT INTO image_colors (file_id, r, g, b, hex, weight) VALUES (?, ?, ?, ?, ?, ?)",
			newFileID, color.R, color.G, color.B, color.Hex, color.Weight)
	}

	query := `
		INSERT INTO metadata_records 
		(filename, provider_name, provider_id, score, file_url, large_file_url, rating, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	execResMatch, err := a.DB.Exec(query,
		filename,
		"danbooru",
		fmt.Sprintf("%d", reqBody.Post.ID),
		100.0,
		reqBody.Post.FileURL,
		reqBody.Post.LargeFileURL,
		reqBody.Post.Rating,
		reqBody.Post.Source,
	)
	if err == nil {
		recordID, _ := execResMatch.LastInsertId()
		saveTags(a.DB, recordID, reqBody.Post.TagsArtist, "artist")
		saveTags(a.DB, recordID, reqBody.Post.TagsCharacters, "character")
		saveTags(a.DB, recordID, reqBody.Post.TagsCopyright, "copyright")
		saveTags(a.DB, recordID, reqBody.Post.TagsGeneral, "general")
		saveTags(a.DB, recordID, reqBody.Post.TagsMeta, "meta")

		a.DB.Exec("UPDATE files SET active_metadata_id = ? WHERE id = ?", recordID, newFileID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"message":      "Successfully downloaded and saved image",
		"new_image_id": newFileID,
		"filename":     filename,
	})
}

func (a *App) handleGetDanbooruSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	username, apiKey := GetDanbooruCredentials(a.DB)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"username": username,
		"api_key":  apiKey,
	})
}

func (a *App) handleSaveDanbooruSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
		APIKey   string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if err := SaveDanbooruCredentials(a.DB, strings.TrimSpace(req.Username), strings.TrimSpace(req.APIKey)); err != nil {
		sendJSONError(w, fmt.Sprintf("Failed to save credentials: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Credentials saved successfully"})
}
