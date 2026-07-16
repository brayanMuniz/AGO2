package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)


func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	var tableExists int
	_ = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='saved_palettes'").Scan(&tableExists)
	isFirstTimePalettes := tableExists == 0

	err = createTables(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	if isFirstTimePalettes {
		_, _ = db.Exec(`INSERT INTO saved_palettes (name, colors) VALUES
			('Catppuccin', '#1e1e2e,#cba6f7,#f38ba8,#a6e3a1,#89b4fa'),
			('Pastel Dream', '#ffb3ba,#ffdfba,#ffffba,#baffc9,#bae1ff')`)
	}

	log.Println("Database successfully initialized!")
	return db, nil
}

func createTables(db *sql.DB) error {
	schema := `
	-- TRACKS THE UNIQUE IMAGE ON DISK
	CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		filename TEXT NOT NULL UNIQUE,
		hash TEXT NOT NULL,
		active_metadata_id INTEGER,
		hasDuplicate INTEGER DEFAULT NULL,
		isFavorite BOOLEAN DEFAULT FALSE,
		organized BOOLEAN DEFAULT FALSE,
		thumbnail_path TEXT DEFAULT NULL,
		image_height INTEGER DEFAULT 0,
		image_width INTEGER DEFAULT 0,
		file_size INTEGER DEFAULT 0,
		
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (active_metadata_id) REFERENCES metadata_records (id) ON DELETE SET NULL,
		FOREIGN KEY (hasDuplicate) REFERENCES files (id) ON DELETE SET NULL
	);

	-- TRACKS THE METADATA SOURCE RECORDS
	CREATE TABLE IF NOT EXISTS metadata_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		filename TEXT NOT NULL,
		
		-- THIRD PARTY TRACKING
		provider_name TEXT NOT NULL,
		provider_id TEXT,
		score REAL DEFAULT 0.0,
		
		-- THE ACTUAL DATA
		file_url TEXT,
		large_file_url TEXT,
		rating TEXT,
		source TEXT,

		-- ORIGINAL SOURCE REFERENCE (populated when customizing a danbooru match)
		original_post_id TEXT DEFAULT NULL,
		original_source TEXT DEFAULT NULL,
		
		FOREIGN KEY (filename) REFERENCES files (filename) ON DELETE CASCADE ON UPDATE CASCADE
	);

	CREATE TABLE IF NOT EXISTS tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		category TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS record_tags (
		metadata_id INTEGER NOT NULL,
		tag_id INTEGER NOT NULL,
		
		PRIMARY KEY (metadata_id, tag_id),
		FOREIGN KEY (metadata_id) REFERENCES metadata_records (id) ON DELETE CASCADE,
		FOREIGN KEY (tag_id) REFERENCES tags (id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS image_colors (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    file_id INTEGER NOT NULL,
	    r INTEGER NOT NULL,
	    g INTEGER NOT NULL,
	    b INTEGER NOT NULL,
	    hex TEXT NOT NULL,
	    weight REAL NOT NULL DEFAULT 0.0,
	    FOREIGN KEY (file_id) REFERENCES files (id) ON DELETE CASCADE
	);

	-- SAVED SEARCH FILTER PRESETS
	CREATE TABLE IF NOT EXISTS saved_filters (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		query TEXT NOT NULL,
		sort_by TEXT DEFAULT 'none',
		sort_order TEXT DEFAULT 'desc',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- SAVED PALETTES
	CREATE TABLE IF NOT EXISTS saved_palettes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		colors TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- APP SETTINGS
	CREATE TABLE IF NOT EXISTS app_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Index for faster lookups when we doing math on colors columns
	CREATE INDEX IF NOT EXISTS idx_image_colors_rgb ON image_colors(r, g, b);

	-- Speeds up "Missing Data", "Duplicate", and "Organized" queries
	CREATE INDEX IF NOT EXISTS idx_files_status ON files(active_metadata_id, hasDuplicate, organized);
	`
	_, err := db.Exec(schema)
	return err
}


type ProcessedImage struct {
	AutoMatch bool
	Skipped   bool
}

// OPTIMIZE: If in the future this is too slow use a transaction instead
func ProcessNewImageUpload(db *sql.DB, apiKey, userName, filename, filePath string) (ProcessedImage, error) {
	result := ProcessedImage{AutoMatch: false, Skipped: false}

	hash, err := GetPixelHash(filePath)
	if err != nil {
		return result, fmt.Errorf("failed to hash image: %w", err)
	}

	imgWidth, imgHeight, fileSize := GetFileDimensionsAndSize(filePath)

	palette, err := ExtractColorPalette(filePath, 5) // Get top 5 colors
	if err != nil {
		fmt.Printf("Warning: failed to extract color palette for %s: %v\n", filename, err)
		palette = []Color{} // Default to empty array on failure
	}

	// First check if a file with this exact filename already exists in the database
	var existingFileID int64
	var existingFileHash string
	err = db.QueryRow("SELECT id, hash FROM files WHERE filename = ?", filename).Scan(&existingFileID, &existingFileHash)
	if err == nil {
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

	thumbPath, thumbErr := GenerateThumbnail(filePath, "thumbnails")
	if thumbErr != nil {
		fmt.Printf("Warning: failed to generate thumbnail for %s: %v\n", filename, thumbErr)
	} else {
		_, err = db.Exec("UPDATE files SET thumbnail_path = ? WHERE filename = ?", thumbPath, filename)
		if err != nil {
			fmt.Printf("Warning: failed to save thumbnail path to DB: %v\n", err)
		}
	}

	matches, err := iqdb_upload_request(apiKey, userName, filePath)
	if err != nil {
		return result, fmt.Errorf("iqdb api failed: %w", err)
	}

	var bestMatch *IQDBMatch
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
		saveTags(db, recordID, bestMatch.Post.TagsArtist, "artist")
		saveTags(db, recordID, bestMatch.Post.TagsCharacters, "character")
		saveTags(db, recordID, bestMatch.Post.TagsCopyright, "copyright")
		saveTags(db, recordID, bestMatch.Post.TagsGeneral, "general")
		saveTags(db, recordID, bestMatch.Post.TagsMeta, "meta")

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

func GetApprovedMetadata(db *sql.DB, filename string) (*Post, error) {
	query := `
		SELECT 
			m.provider_id, m.file_url, m.large_file_url, m.rating, 
			m.source, f.image_height, f.image_width, f.file_size
		FROM files f
		JOIN metadata_records m ON f.active_metadata_id = m.id
		WHERE f.filename = ?
	`

	var post Post
	var providerID string

	err := db.QueryRow(query, filename).Scan(
		&providerID,
		&post.FileURL,
		&post.LargeFileURL,
		&post.Rating,
		&post.Source,
		&post.ImageHeight,
		&post.ImageWidth,
		&post.FileSize,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no verified record found for %s (it might be in the verification queue)", filename)
		}
		return nil, err
	}

	fmt.Sscanf(providerID, "%d", &post.ID)

	return &post, nil
}


// inserts tags into the dictionary and links them to the metadata record
func saveTags(db *sql.DB, recordID int64, tags []string, category string) {
	for _, tagName := range tags {
		if strings.TrimSpace(tagName) == "" {
			continue
		}

		_, err := db.Exec("INSERT OR IGNORE INTO tags (name, category) VALUES (?, ?)", tagName, category)
		if err != nil {
			fmt.Printf("Error inserting tag '%s': %v\n", tagName, err)
			continue
		}

		var tagID int64
		err = db.QueryRow("SELECT id FROM tags WHERE name = ?", tagName).Scan(&tagID)
		if err != nil {
			fmt.Printf("Error fetching tag ID for '%s': %v\n", tagName, err)
			continue
		}

		_, err = db.Exec("INSERT OR IGNORE INTO record_tags (metadata_id, tag_id) VALUES (?, ?)", recordID, tagID)
		if err != nil {
			fmt.Printf("Error linking tag '%s' to record %d: %v\n", tagName, recordID, err)
		}
	}
}

func GetImageByID(db *sql.DB, fileID int64, includeMatches bool) (*Image, error) {
	var img Image
	var activeMetadataID sql.NullInt64
	var hasDuplicate sql.NullInt64
	img.ID = fileID

	err := db.QueryRow("SELECT filename, hash, isFavorite, organized, active_metadata_id, IFNULL(thumbnail_path, ''), hasDuplicate, image_width, image_height, file_size FROM files WHERE id = ?", fileID).
		Scan(&img.FileName, &img.Hash, &img.IsFavorite, &img.Organized, &activeMetadataID, &img.ThumbnailPath, &hasDuplicate, &img.ImageWidth, &img.ImageHeight, &img.FileSize)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no file found with ID %d", fileID)
		}
		return nil, fmt.Errorf("failed to fetch file data: %w", err)
	}

	if hasDuplicate.Valid {
		val := hasDuplicate.Int64
		img.HasDuplicate = &val
	}

	if !activeMetadataID.Valid {
		return &img, nil
	}

	query := `
	    SELECT id, provider_id, score, file_url, large_file_url, rating, source,
	           IFNULL(original_post_id, ''), IFNULL(original_source, '')
	    FROM metadata_records 
	    WHERE id = ?`

	rows, err := db.Query(query, activeMetadataID.Int64)
	if err != nil {
		return nil, fmt.Errorf("failed to query metadata records: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var recordID int64
		var providerID string
		var score sql.NullFloat64
		var post Post

		var rating, source, fileURL, largeFileURL sql.NullString

		err := rows.Scan(
			&recordID, &providerID, &score, &fileURL, &largeFileURL, &rating,
			&source, &post.OriginalPostID, &post.OriginalSource,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan metadata row: %w", err)
		}

		fmt.Sscanf(providerID, "%d", &post.ID)
		post.FileURL = fileURL.String
		post.LargeFileURL = largeFileURL.String
		post.Rating = rating.String
		post.Source = source.String
		post.ImageHeight = img.ImageHeight
		post.ImageWidth = img.ImageWidth
		post.FileSize = int(img.FileSize)

		err = populateTags(db, recordID, &post)
		if err != nil {
			return nil, fmt.Errorf("failed to populate tags for record: %w", err)
		}

		mainPost := post
		img.MainData = &mainPost
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating metadata rows: %w", err)
	}

	return &img, nil
}

// fetches tags for a specific metadata record and organizes them into the Post struct
func populateTags(db *sql.DB, metadataID int64, post *Post) error {
	query := `
		SELECT t.name, t.category 
		FROM tags t
		JOIN record_tags rt ON t.id = rt.tag_id
		WHERE rt.metadata_id = ?
	`
	rows, err := db.Query(query, metadataID)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Initialize slices to prevent null in JSON
	post.TagsArtist = make([]string, 0)
	post.TagsCharacters = make([]string, 0)
	post.TagsCopyright = make([]string, 0)
	post.TagsGeneral = make([]string, 0)
	post.TagsMeta = make([]string, 0)

	for rows.Next() {
		var name, category string
		if err := rows.Scan(&name, &category); err != nil {
			return err
		}

		switch category {
		case "artist":
			post.TagsArtist = append(post.TagsArtist, name)
		case "character":
			post.TagsCharacters = append(post.TagsCharacters, name)
		case "copyright":
			post.TagsCopyright = append(post.TagsCopyright, name)
		case "general":
			post.TagsGeneral = append(post.TagsGeneral, name)
		case "meta":
			post.TagsMeta = append(post.TagsMeta, name)
		}
	}

	post.TagCountArtist = len(post.TagsArtist)
	post.TagCountCharacter = len(post.TagsCharacters)
	post.TagCountCopyright = len(post.TagsCopyright)
	post.TagCountGeneral = len(post.TagsGeneral)
	post.TagCountMeta = len(post.TagsMeta)
	post.TagCount = post.TagCountArtist + post.TagCountCharacter + post.TagCountCopyright + post.TagCountGeneral + post.TagCountMeta

	return rows.Err()
}


func DeleteImageByID(db *sql.DB, fileID int64, galleryDir string) error {
	var filename string
	var thumbnailPath sql.NullString

	err := db.QueryRow("SELECT filename, thumbnail_path FROM files WHERE id = ?", fileID).
		Scan(&filename, &thumbnailPath)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no file found with ID %d", fileID)
		}
		return fmt.Errorf("failed to fetch file data for deletion: %w", err)
	}

	_, err = db.Exec("DELETE FROM files WHERE id = ?", fileID)
	if err != nil {
		return fmt.Errorf("failed to delete database record: %w", err)
	}

	imagePath := filepath.Join(galleryDir, filename)
	if err := os.Remove(imagePath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: failed to delete physical image file %s: %v\n", imagePath, err)
	}

	if thumbnailPath.Valid && thumbnailPath.String != "" {
		if err := os.Remove(thumbnailPath.String); err != nil && !os.IsNotExist(err) {
			fmt.Printf("Warning: failed to delete physical thumbnail file %s: %v\n", thumbnailPath.String, err)
		}
	}

	fmt.Printf("Successfully deleted image record and files for: %s\n", filename)
	return nil
}

type UpdateImageParams struct {
	IsFavorite       *bool
	ActiveMetadataID *int64 // Use 0 or a negative number to clear the metadata
	MainData         *Post
	ReplaceImage     *bool
}

func UpdateImage(db *sql.DB, fileID int64, params UpdateImageParams) error {
	// 1. Verify image exists and get the filename (required for metadata_records)
	var filename string
	err := db.QueryRow("SELECT filename FROM files WHERE id = ?", fileID).Scan(&filename)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no file found with ID %d", fileID)
		}
		return fmt.Errorf("failed to check image existence: %w", err)
	}

	var setClauses []string
	var args []any

	if params.IsFavorite != nil {
		setClauses = append(setClauses, "isFavorite = ?")
		args = append(args, *params.IsFavorite)
	}

	// 2. If we receive main_data, insert it into metadata_records
	if params.MainData != nil {
		post := params.MainData

		providerName := "danbooru"
		if strings.EqualFold(post.Source, "Custom") {
			providerName = "custom"
		}

		// Look up the current active metadata to preserve original source reference
		var origPostID, origSource sql.NullString
		if providerName == "custom" {
			// If this file already had danbooru metadata, capture it as the original reference
			row := db.QueryRow(`
				SELECT m.provider_id, m.source, m.original_post_id, m.original_source
				FROM files f
				JOIN metadata_records m ON f.active_metadata_id = m.id
				WHERE f.id = ?`, fileID)

			var prevProviderID, prevSource, prevOrigPostID, prevOrigSource sql.NullString
			if err := row.Scan(&prevProviderID, &prevSource, &prevOrigPostID, &prevOrigSource); err == nil {
				if prevOrigPostID.Valid && prevOrigPostID.String != "" {
					// Already customized before — carry forward the existing original reference
					origPostID = prevOrigPostID
					origSource = prevOrigSource
				} else if prevProviderID.Valid && prevProviderID.String != "" && prevProviderID.String != "0" {
					// First customization of a danbooru match — snapshot the current provider data
					origPostID = prevProviderID
					origSource = prevSource
				}
			}
		}

		query := `
			INSERT INTO metadata_records 
			(filename, provider_name, provider_id, score, file_url, large_file_url, rating, source, original_post_id, original_source)
			VALUES (?, ?, ?, 100.0, ?, ?, ?, ?, ?, ?)
		`

		execRes, err := db.Exec(query,
			filename,
			providerName,
			fmt.Sprintf("%d", post.ID),
			post.FileURL,
			post.LargeFileURL,
			post.Rating,
			post.Source,
			origPostID,
			origSource,
		)
		if err != nil {
			return fmt.Errorf("failed to insert metadata record: %w", err)
		}

		// Grab the internal auto-incremented ID
		newRecordID, _ := execRes.LastInsertId()

		// Save all the tags
		saveTags(db, newRecordID, post.TagsArtist, "artist")
		saveTags(db, newRecordID, post.TagsCharacters, "character")
		saveTags(db, newRecordID, post.TagsCopyright, "copyright")
		saveTags(db, newRecordID, post.TagsGeneral, "general")
		saveTags(db, newRecordID, post.TagsMeta, "meta")

		// Tell the files table to use this new internal ID and mark as organized
		setClauses = append(setClauses, "active_metadata_id = ?", "organized = TRUE")
		args = append(args, newRecordID)

	} else if params.ActiveMetadataID != nil {
		// Fallback for clearing metadata or using an existing ID
		id := *params.ActiveMetadataID
		if id <= 0 {
			setClauses = append(setClauses, "active_metadata_id = NULL")
		} else {
			setClauses = append(setClauses, "active_metadata_id = ?")
			args = append(args, id)
		}
	}

	if len(setClauses) == 0 {
		return nil
	}

	// 3. Finalize the file update
	query := fmt.Sprintf("UPDATE files SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	args = append(args, fileID)

	_, err = db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to execute dynamic file update: %w", err)
	}

	return nil
}

func GetDanbooruCredentials(db *sql.DB) (string, string) {
	var username, apiKey string
	if db != nil {
		_ = db.QueryRow("SELECT value FROM app_settings WHERE key = 'danbooru_username'").Scan(&username)
		_ = db.QueryRow("SELECT value FROM app_settings WHERE key = 'danbooru_api_key'").Scan(&apiKey)
	}
	if username == "" {
		username = os.Getenv("USERNAME")
	}
	if apiKey == "" {
		apiKey = os.Getenv("DANBOORU_KEY")
	}
	return username, apiKey
}

func SaveDanbooruCredentials(db *sql.DB, username, apiKey string) error {
	_, err := db.Exec("INSERT INTO app_settings (key, value, updated_at) VALUES ('danbooru_username', ?, CURRENT_TIMESTAMP) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP", username)
	if err != nil {
		return err
	}
	_, err = db.Exec("INSERT INTO app_settings (key, value, updated_at) VALUES ('danbooru_api_key', ?, CURRENT_TIMESTAMP) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP", apiKey)
	if err != nil {
		return err
	}

	os.Setenv("USERNAME", username)
	os.Setenv("DANBOORU_KEY", apiKey)
	updateEnvFile("USERNAME", username)
	updateEnvFile("DANBOORU_KEY", apiKey)
	return nil
}

func updateEnvFile(key, value string) {
	envPath := "./.env"
	content, err := os.ReadFile(envPath)
	lines := []string{}
	if err == nil {
		lines = strings.Split(string(content), "\n")
	}
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), key+"=") {
			lines[i] = fmt.Sprintf("%s=%s", key, value)
			found = true
			break
		}
	}
	if !found {
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines[len(lines)-1] = fmt.Sprintf("%s=%s", key, value)
			lines = append(lines, "")
		} else {
			lines = append(lines, fmt.Sprintf("%s=%s", key, value))
		}
	}
	_ = os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0644)
}



