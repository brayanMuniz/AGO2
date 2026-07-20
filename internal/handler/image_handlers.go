package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/brayanMuniz/AGO2/internal/danbooru"
	"github.com/brayanMuniz/AGO2/internal/gallery"
	"github.com/brayanMuniz/AGO2/internal/models"
	"github.com/brayanMuniz/AGO2/internal/storage"
)

// GET /api/image/{id}
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

	img, err := storage.GetImageByID(a.DB, id, true)
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

// GET /api/image/{id}/matches
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

	img, err := storage.GetImageByID(a.DB, id, false)
	if err != nil {
		if strings.Contains(err.Error(), "no file found") {
			sendJSONError(w, err.Error(), http.StatusNotFound)
		} else {
			sendJSONError(w, "Failed to fetch image", http.StatusInternalServerError)
			fmt.Printf("Database error fetching image %d: %v\n", id, err)
		}
		return
	}

	userName, apiKey := danbooru.GetCredentials(a.DB)
	fileLocation := filepath.Join(a.Cfg.GalleryDir, img.FileName)

	matches, err := danbooru.IQDBUploadRequest(apiKey, userName, fileLocation)
	if err != nil {
		sendJSONError(w, "Failed to fetch image data", http.StatusInternalServerError)
		fmt.Printf("Could not get iqdb request for %d", id)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(matches)
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
		IsFavorite       *bool       `json:"is_favorite,omitempty"`
		ActiveMetadataID *int64      `json:"active_metadata_id,omitempty"`
		MainData         *models.Post `json:"main_data,omitempty"`
		ReplaceImage     *bool       `json:"replace_image,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		sendJSONError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	params := models.UpdateImageParams{
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

	err = storage.UpdateImage(a.DB, id, params)
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
			userName, apiKey := danbooru.GetCredentials(a.DB)
			destPath := filepath.Join(a.Cfg.GalleryDir, filename)
			err := gallery.DownloadAndReplaceImage(userName, apiKey, targetURL, destPath)
			if err != nil {
				fmt.Printf("Error replacing physical file %s: %v\n", filename, err)
				sendJSONError(w, fmt.Sprintf("Failed to download and replace file: %v", err), http.StatusInternalServerError)
				return
			}
			fmt.Printf("Successfully replaced physical file for: %s\n", filename)
			dimErr := gallery.UpdateFileDimensionsInDB(a.DB, filename, destPath, a.Cfg.ThumbnailDir)
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

	err = storage.DeleteImageByID(a.DB, id, a.Cfg.GalleryDir)
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

	var failedIDs []int64

	// Loop through and delete
	for _, id := range req.IDs {
		err := storage.DeleteImageByID(a.DB, id, a.Cfg.GalleryDir)
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

// POST /api/image/download-match
func (a *App) handleDownloadMatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqBody struct {
		Post *models.Post `json:"post"`
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
	destPath := filepath.Join(a.Cfg.GalleryDir, filename)

	counter := 1
	for {
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			break
		}
		filename = fmt.Sprintf("danbooru_%d_%d%s", reqBody.Post.ID, counter, ext)
		destPath = filepath.Join(a.Cfg.GalleryDir, filename)
		counter++
	}

	userName, apiKey := danbooru.GetCredentials(a.DB)
	if err := gallery.DownloadAndReplaceImage(userName, apiKey, targetURL, destPath); err != nil {
		sendJSONError(w, fmt.Sprintf("Failed to download image: %v", err), http.StatusInternalServerError)
		return
	}

	hash, err := gallery.GetPixelHash(destPath)
	if err != nil {
		os.Remove(destPath)
		sendJSONError(w, "Failed to hash downloaded image", http.StatusInternalServerError)
		return
	}
	imgWidth, imgHeight, fileSize := gallery.GetFileDimensionsAndSize(destPath)

	execRes, err := a.DB.Exec("INSERT INTO files (filename, hash, isFavorite, organized, image_width, image_height, file_size) VALUES (?, ?, FALSE, TRUE, ?, ?, ?)", filename, hash, imgWidth, imgHeight, fileSize)
	if err != nil {
		os.Remove(destPath)
		sendJSONError(w, "Failed to save file record to DB", http.StatusInternalServerError)
		return
	}
	newFileID, _ := execRes.LastInsertId()

	thumbPath, thumbErr := gallery.GenerateThumbnail(destPath, a.Cfg.ThumbnailDir)
	if thumbErr == nil {
		a.DB.Exec("UPDATE files SET thumbnail_path = ? WHERE id = ?", thumbPath, newFileID)
	}

	palette, _ := gallery.ExtractColorPalette(destPath, 5)
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
		storage.SaveTags(a.DB, recordID, reqBody.Post.TagsArtist, "artist")
		storage.SaveTags(a.DB, recordID, reqBody.Post.TagsCharacters, "character")
		storage.SaveTags(a.DB, recordID, reqBody.Post.TagsCopyright, "copyright")
		storage.SaveTags(a.DB, recordID, reqBody.Post.TagsGeneral, "general")
		storage.SaveTags(a.DB, recordID, reqBody.Post.TagsMeta, "meta")

		a.DB.Exec("UPDATE files SET active_metadata_id = ? WHERE id = ?", recordID, newFileID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"message":      "Successfully downloaded and saved image",
		"new_image_id": newFileID,
		"filename":     filename,
	})
}
