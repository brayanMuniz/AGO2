package gallery

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/brayanMuniz/AGO2/internal/danbooru"
	"github.com/brayanMuniz/AGO2/internal/models"
	"github.com/brayanMuniz/AGO2/internal/storage"
)

// --- Job Store ---

// WARNING: I am not currently cleaning up this job
var (
	jobStoreMu sync.RWMutex
	jobStore   = make(map[string]*models.JobState)
)

// CreateJob creates a new gallery processing job and registers it in the store.
func CreateJob() *models.JobState {
	job := &models.JobState{
		ID:     generateJobID(),
		Status: "processing",
		Stats:  models.ProcessGallerySum{Processed: 0, AutoMatch: 0, Skipped: 0},
	}

	jobStoreMu.Lock()
	jobStore[job.ID] = job
	jobStoreMu.Unlock()

	return job
}

// GetJob retrieves a job by its ID.
func GetJob(id string) (*models.JobState, bool) {
	jobStoreMu.RLock()
	job, exists := jobStore[id]
	jobStoreMu.RUnlock()
	return job, exists
}

func generateJobID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// --- Gallery Worker ---

// RunGalleryWorker scans a directory for images, processes new/changed files,
// and attempts to auto-match them against the Danbooru IQDB API.
func RunGalleryWorker(db *sql.DB, apikey, userName, dirPath, thumbnailDir string, job *models.JobState) {
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

		r, err := ProcessNewImageUpload(db, apikey, userName, fileName, filePath, thumbnailDir)
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

// --- Image Processing ---

// ProcessNewImageUpload handles a single image file: hashes it, checks for duplicates,
// extracts colors, generates a thumbnail, and queries IQDB for auto-matching.
// OPTIMIZE: If in the future this is too slow use a transaction instead
func ProcessNewImageUpload(db *sql.DB, apiKey, userName, filename, filePath, thumbnailDir string) (models.ProcessedImage, error) {
	result := models.ProcessedImage{AutoMatch: false, Skipped: false}

	// First check fast-path if a file with this exact filename exists and can be skipped without hashing or reading image bytes
	var existingFileID int64
	var existingFileHash string
	var existingFileSize int64
	var existingOrganized bool
	err := db.QueryRow("SELECT id, hash, file_size, organized FROM files WHERE filename = ?", filename).Scan(&existingFileID, &existingFileHash, &existingFileSize, &existingOrganized)
	if err == nil {
		info, statErr := os.Stat(filePath)
		if statErr == nil && info.Size() == existingFileSize && existingFileSize > 0 {
			if existingOrganized || existingFileHash != "" {
				fmt.Printf("Skipped: %s is already processed.\n", filename)
				result.Skipped = true
				return result, nil
			}
		}
	}

	hash, err := GetPixelHash(filePath)
	if err != nil {
		return result, fmt.Errorf("failed to hash image: %w", err)
	}

	imgWidth, imgHeight, fileSize := GetFileDimensionsAndSize(filePath)

	palette, err := ExtractColorPalette(filePath, 5) // Get top 5 colors
	if err != nil {
		fmt.Printf("Warning: failed to extract color palette for %s: %v\n", filename, err)
		palette = []models.Color{} // Default to empty array on failure
	}

	if existingFileID > 0 {
		if existingFileHash == hash {
			// Filename and hash match. Already processed.
			fmt.Printf("Skipped: %s is already processed.\n", filename)
			result.Skipped = true
			return result, nil
		}

		// Filename exists but file was modified/replaced on disk with a new version.
		fmt.Printf("Updating modified file record for: %s\n", filename)
		_, err = db.Exec("UPDATE files SET hash = ?, image_width = ?, image_height = ?, file_size = ? WHERE id = ?",
			hash, imgWidth, imgHeight, fileSize, existingFileID)
		if err != nil {
			return result, fmt.Errorf("failed to update modified file record: %w", err)
		}

		db.Exec("DELETE FROM image_colors WHERE file_id = ?", existingFileID)
		for _, color := range palette {
			db.Exec("INSERT INTO image_colors (file_id, r, g, b, hex, weight) VALUES (?, ?, ?, ?, ?, ?)",
				existingFileID, color.R, color.G, color.B, color.Hex, color.Weight)
		}

		result.Skipped = true
		return result, nil
	}

	// Checks for an existing original file with the same pixel hash
	var existingID int64
	var existingFilename string
	err = db.QueryRow("SELECT id, filename FROM files WHERE hash = ? AND hasDuplicate IS NULL", hash).Scan(&existingID, &existingFilename)

	if err == nil {
		// Same hash AND same filename. Skip entirely.
		if filename == existingFilename {
			fmt.Printf("Skipped: %s is already processed.\n", filename)
			result.Skipped = true
			return result, nil
		}

		// Same hash BUT different filename. Log as duplicate.
		fmt.Printf("Duplicate: %s is a copy of %s\n", filename, existingFilename)
		_, err = db.Exec("INSERT INTO files (filename, hash, hasDuplicate, isFavorite, image_width, image_height, file_size) VALUES (?, ?, ?, FALSE, ?, ?, ?)", filename, hash, existingID, imgWidth, imgHeight, fileSize)
		if err != nil {
			return result, fmt.Errorf("failed to insert duplicate file record: %w", err)
		}

		result.Skipped = true
		return result, nil
	}

	// Insert new original file
	execRes, err := db.Exec("INSERT INTO files (filename, hash, isFavorite, image_width, image_height, file_size) VALUES (?, ?, FALSE, ?, ?, ?)", filename, hash, imgWidth, imgHeight, fileSize)
	if err != nil {
		return result, fmt.Errorf("failed to insert file record: %w", err)
	}
	newFileID, _ := execRes.LastInsertId()
	fmt.Printf("Saved new file record: %s\n", filename)

	// Save the extracted colors to the linked table
	for _, color := range palette {
		_, err = db.Exec(
			"INSERT INTO image_colors (file_id, r, g, b, hex, weight) VALUES (?, ?, ?, ?, ?, ?)",
			newFileID, color.R, color.G, color.B, color.Hex, color.Weight,
		)
		if err != nil {
			fmt.Printf("Warning: failed to insert color for %s: %v\n", filename, err)
		}
	}

	thumbPath, thumbErr := GenerateThumbnail(filePath, thumbnailDir)
	if thumbErr != nil {
		fmt.Printf("Warning: failed to generate thumbnail for %s: %v\n", filename, thumbErr)
	} else {
		_, err = db.Exec("UPDATE files SET thumbnail_path = ? WHERE filename = ?", thumbPath, filename)
		if err != nil {
			fmt.Printf("Warning: failed to save thumbnail path to DB: %v\n", err)
		}
	}

	matches, err := danbooru.IQDBUploadRequest(apiKey, userName, filePath)
	if err != nil {
		return result, fmt.Errorf("iqdb api failed: %w", err)
	}

	var bestMatch *models.IQDBMatch
	for i := range matches {
		if matches[i].Score >= 95.0 {
			if bestMatch == nil || matches[i].Score > bestMatch.Score {
				bestMatch = &matches[i]
			}
		}
	}

	// Only save metadata if we found an auto-verify 95%+ match
	if bestMatch != nil {
		query := `
			INSERT INTO metadata_records 
			(filename, provider_name, provider_id, score, file_url, large_file_url, rating, source)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`
		execResMatch, err := db.Exec(query,
			filename,
			"danbooru",
			fmt.Sprintf("%d", bestMatch.Post.ID),
			bestMatch.Score,
			bestMatch.Post.FileURL,
			bestMatch.Post.LargeFileURL,
			bestMatch.Post.Rating,
			bestMatch.Post.Source,
		)
		if err != nil {
			return result, fmt.Errorf("failed to save best match record: %w", err)
		}

		recordID, _ := execResMatch.LastInsertId()
		storage.SaveTags(db, recordID, bestMatch.Post.TagsArtist, "artist")
		storage.SaveTags(db, recordID, bestMatch.Post.TagsCharacters, "character")
		storage.SaveTags(db, recordID, bestMatch.Post.TagsCopyright, "copyright")
		storage.SaveTags(db, recordID, bestMatch.Post.TagsGeneral, "general")
		storage.SaveTags(db, recordID, bestMatch.Post.TagsMeta, "meta")

		_, err = db.Exec("UPDATE files SET active_metadata_id = ?, organized = TRUE WHERE filename = ?", recordID, filename)
		if err != nil {
			return result, fmt.Errorf("failed to lock in active metadata: %w", err)
		}
		result.AutoMatch = true
	} else {
		fmt.Printf("No 95%%+ match found for %s. Unmatched image stored without unused metadata matches.\n", filename)
	}

	return result, nil
}

// --- Image Download ---

// DownloadAndReplaceImage downloads an image from a URL and saves it to the destination path.
// It tries the provided credentials for authentication if the initial request fails.
func DownloadAndReplaceImage(userName, apiKey, urlStr, destPath string) error {
	client := &http.Client{Timeout: 45 * time.Second}

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
