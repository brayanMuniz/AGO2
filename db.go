package main

import (
	"database/sql"
	"fmt"
	"image"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// holds the usage statistics for a single tag
type TagStat struct {
	Name     string
	Category string
	Count    int
}

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

	-- Index for faster lookups when we doing math on colors columns
	CREATE INDEX IF NOT EXISTS idx_image_colors_rgb ON image_colors(r, g, b);

	-- Speeds up "Missing Data", "Duplicate", and "Organized" queries
	CREATE INDEX IF NOT EXISTS idx_files_status ON files(active_metadata_id, hasDuplicate, organized);
	`
	_, err := db.Exec(schema)
	return err
}

type SavedFilter struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Query     string `json:"query"`
	SortBy    string `json:"sort_by"`
	SortOrder string `json:"sort_order"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func GetAllSavedFilters(db *sql.DB) ([]SavedFilter, error) {
	rows, err := db.Query("SELECT id, name, query, COALESCE(sort_by, 'none'), COALESCE(sort_order, 'desc'), created_at, updated_at FROM saved_filters ORDER BY name ASC")
	if err != nil {
		return nil, fmt.Errorf("failed to query saved filters: %w", err)
	}
	defer rows.Close()

	var filters []SavedFilter
	for rows.Next() {
		var f SavedFilter
		if err := rows.Scan(&f.ID, &f.Name, &f.Query, &f.SortBy, &f.SortOrder, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan saved filter: %w", err)
		}
		filters = append(filters, f)
	}
	return filters, rows.Err()
}

func CreateSavedFilter(db *sql.DB, name, query, sortBy, sortOrder string) (*SavedFilter, error) {
	if sortBy == "" {
		sortBy = "none"
	}
	if sortOrder == "" {
		sortOrder = "desc"
	}
	result, err := db.Exec(
		"INSERT INTO saved_filters (name, query, sort_by, sort_order) VALUES (?, ?, ?, ?)", name, query, sortBy, sortOrder,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create saved filter: %w", err)
	}
	id, _ := result.LastInsertId()

	var f SavedFilter
	err = db.QueryRow("SELECT id, name, query, COALESCE(sort_by, 'none'), COALESCE(sort_order, 'desc'), created_at, updated_at FROM saved_filters WHERE id = ?", id).
		Scan(&f.ID, &f.Name, &f.Query, &f.SortBy, &f.SortOrder, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created filter: %w", err)
	}
	return &f, nil
}

func UpdateSavedFilter(db *sql.DB, id int64, name, query, sortBy, sortOrder string) (*SavedFilter, error) {
	if sortBy == "" {
		sortBy = "none"
	}
	if sortOrder == "" {
		sortOrder = "desc"
	}
	_, err := db.Exec(
		"UPDATE saved_filters SET name = ?, query = ?, sort_by = ?, sort_order = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		name, query, sortBy, sortOrder, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update saved filter: %w", err)
	}

	var f SavedFilter
	err = db.QueryRow("SELECT id, name, query, COALESCE(sort_by, 'none'), COALESCE(sort_order, 'desc'), created_at, updated_at FROM saved_filters WHERE id = ?", id).
		Scan(&f.ID, &f.Name, &f.Query, &f.SortBy, &f.SortOrder, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated filter: %w", err)
	}
	return &f, nil
}

func DeleteSavedFilter(db *sql.DB, id int64) error {
	result, err := db.Exec("DELETE FROM saved_filters WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete saved filter: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("no saved filter found with ID %d", id)
	}
	return nil
}

type SavedPalette struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Colors    []string `json:"colors"`
	CreatedAt string   `json:"created_at"`
}

func GetSavedPalettes(db *sql.DB) ([]SavedPalette, error) {
	rows, err := db.Query("SELECT id, name, colors, created_at FROM saved_palettes ORDER BY name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var palettes []SavedPalette
	for rows.Next() {
		var p SavedPalette
		var colorsStr string
		if err := rows.Scan(&p.ID, &p.Name, &colorsStr, &p.CreatedAt); err != nil {
			continue
		}
		for _, c := range strings.Split(colorsStr, ",") {
			cClean := strings.TrimSpace(c)
			if cClean != "" {
				p.Colors = append(p.Colors, cClean)
			}
		}
		palettes = append(palettes, p)
	}
	return palettes, nil
}

func CreateSavedPalette(db *sql.DB, name string, colors []string) (*SavedPalette, error) {
	nameClean := strings.TrimSpace(name)
	if nameClean == "" {
		return nil, fmt.Errorf("palette name cannot be empty")
	}
	if len(colors) == 0 {
		return nil, fmt.Errorf("palette must contain at least one color")
	}
	colorsStr := strings.Join(colors, ",")

	res, err := db.Exec("INSERT INTO saved_palettes (name, colors) VALUES (?, ?)", nameClean, colorsStr)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &SavedPalette{
		ID:     id,
		Name:   nameClean,
		Colors: colors,
	}, nil
}

func DeleteSavedPalette(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM saved_palettes WHERE id = ?", id)
	return err
}

type ProcessedImage struct {
	AutoMatch bool
	Skipped   bool
}

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
func UpdateFileDimensionsInDB(db *sql.DB, filename string, filePath string) error {
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
	GenerateThumbnail(filePath, "thumbnails")
	return nil
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

func GetActiveTagStats(db *sql.DB, limit int) ([]TagStat, error) {
	query := `
		SELECT t.name, t.category, COUNT(rt.metadata_id) as usage_count
		FROM tags t
		JOIN record_tags rt ON t.id = rt.tag_id
		JOIN files f ON f.active_metadata_id = rt.metadata_id
		GROUP BY t.id
		ORDER BY usage_count DESC
	`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tag stats: %w", err)
	}
	defer rows.Close()

	var stats []TagStat

	for rows.Next() {
		var stat TagStat
		if err := rows.Scan(&stat.Name, &stat.Category, &stat.Count); err != nil {
			return nil, fmt.Errorf("failed to scan tag row: %w", err)
		}
		stats = append(stats, stat)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return stats, nil
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

func SearchImagesByTags(db *sql.DB, searchTokens []string) ([]Image, error) {
	if len(searchTokens) == 0 {
		return nil, fmt.Errorf("no search tokens provided")
	}

	var normalTags []string
	var negatedTags []string
	var ratings []string
	var isFavoriteFilter *bool
	var orderByColor string
	var targetColors []Color

	// Base query joining files to metadata_records
	query := "SELECT f.id FROM files f LEFT JOIN metadata_records m ON f.active_metadata_id = m.id"
	var whereClauses []string
	var args []interface{}

	parseNumericToken := func(column, token, prefix string) error {
		valStr := strings.TrimPrefix(token, prefix)
		op := "="

		if strings.HasPrefix(valStr, ">=") {
			op = ">="
			valStr = valStr[2:]
		} else if strings.HasPrefix(valStr, "<=") {
			op = "<="
			valStr = valStr[2:]
		} else if strings.HasPrefix(valStr, ">") {
			op = ">"
			valStr = valStr[1:]
		} else if strings.HasPrefix(valStr, "<") {
			op = "<"
			valStr = valStr[1:]
		}

		val, err := strconv.Atoi(valStr)
		if err != nil {
			return fmt.Errorf("invalid numeric value in search token: '%s'", token)
		}

		whereClauses = append(whereClauses, fmt.Sprintf("f.%s %s ?", column, op))
		args = append(args, val)
		return nil
	}

	// Separate tokens into categories
	for _, token := range searchTokens {
		lowerToken := strings.ToLower(strings.TrimSpace(token))

		if strings.HasPrefix(lowerToken, "rating:") {
			ratings = append(ratings, strings.TrimPrefix(lowerToken, "rating:"))
		} else if strings.HasPrefix(lowerToken, "width:") {
			if err := parseNumericToken("image_width", lowerToken, "width:"); err != nil {
				return nil, err
			}
		} else if strings.HasPrefix(lowerToken, "height:") {
			if err := parseNumericToken("image_height", lowerToken, "height:"); err != nil {
				return nil, err
			}
		} else if strings.HasPrefix(lowerToken, "size:") {
			if err := parseNumericToken("file_size", lowerToken, "size:"); err != nil {
				return nil, err
			}
		} else if strings.HasPrefix(lowerToken, "favorite:") {
			valStr := strings.TrimPrefix(lowerToken, "favorite:")
			favVal, err := strconv.ParseBool(valStr)
			if err != nil {
				return nil, fmt.Errorf("invalid boolean value in search token: '%s'", token)
			}
			isFavoriteFilter = &favVal

		} else if strings.HasPrefix(lowerToken, "brightness:") {
			valStr := strings.TrimPrefix(lowerToken, "brightness:")
			parts := strings.Split(valStr, "-")
			if len(parts) == 2 {
				minB, err1 := strconv.ParseFloat(parts[0], 64)
				maxB, err2 := strconv.ParseFloat(parts[1], 64)
				if err1 == nil && err2 == nil {
					// Use a subquery to calculate the average brightness of the image's palette
					whereClauses = append(whereClauses, `
						(SELECT AVG((0.299 * r) + (0.587 * g) + (0.114 * b)) 
						 FROM image_colors WHERE file_id = f.id) BETWEEN ? AND ?
					`)
					args = append(args, minB, maxB)
				}
			}

		} else if strings.HasPrefix(lowerToken, "color:") {
			hexStr := strings.TrimPrefix(lowerToken, "color:")
			for _, p := range strings.Split(hexStr, ",") {
				pClean := strings.TrimSpace(p)
				if pClean != "" {
					targetColors = append(targetColors, ParseHexToColor(pClean))
				}
			}
		} else if strings.HasPrefix(lowerToken, "palette:") {
			hexStr := strings.TrimPrefix(lowerToken, "palette:")
			for _, p := range strings.Split(hexStr, ",") {
				pClean := strings.TrimSpace(p)
				if pClean != "" {
					targetColors = append(targetColors, ParseHexToColor(pClean))
				}
			}
		} else if lowerToken == "is:missing" {
			whereClauses = append(whereClauses, "f.active_metadata_id IS NULL AND f.hasDuplicate IS NULL")
		} else if lowerToken == "is:duplicate" {
			whereClauses = append(whereClauses, "f.hasDuplicate IS NOT NULL")
		} else if lowerToken == "is:organized" {
			whereClauses = append(whereClauses, "f.organized = TRUE")
		} else if lowerToken == "is:unorganized" {
			whereClauses = append(whereClauses, "f.organized = FALSE")
		} else if strings.HasPrefix(lowerToken, "-") && len(lowerToken) > 1 {
			negatedTags = append(negatedTags, strings.TrimSpace(token[1:]))
		} else {
			normalTags = append(normalTags, strings.TrimSpace(token))
		}
	}

	// Handle Rating Filters
	if len(ratings) > 0 {
		placeholders := make([]string, len(ratings))
		for i, r := range ratings {
			placeholders[i] = "?"
			args = append(args, r)
		}
		whereClauses = append(whereClauses, fmt.Sprintf("LOWER(m.rating) IN (%s)", strings.Join(placeholders, ",")))
	}

	// Handle Favorite Filter
	if isFavoriteFilter != nil {
		whereClauses = append(whereClauses, "f.isFavorite = ?")
		args = append(args, *isFavoriteFilter)
	}

	// Handle Tag Filters
	if len(normalTags) > 0 {
		query += `
			JOIN record_tags rt ON m.id = rt.metadata_id
			JOIN tags t ON rt.tag_id = t.id`

		placeholders := make([]string, len(normalTags))
		for i, tag := range normalTags {
			placeholders[i] = "?"
			args = append(args, tag)
		}
		whereClauses = append(whereClauses, fmt.Sprintf("t.name IN (%s)", strings.Join(placeholders, ",")))
	}

	// Handle Negated Tag Filters (exclude images that have these tags)
	for _, negTag := range negatedTags {
		whereClauses = append(whereClauses, `NOT EXISTS (
			SELECT 1 FROM record_tags nrt
			JOIN tags nt ON nrt.tag_id = nt.id
			WHERE nrt.metadata_id = m.id AND nt.name = ?
		)`)
		args = append(args, negTag)
	}

	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	if len(normalTags) > 0 {
		query += " GROUP BY f.id HAVING COUNT(DISTINCT t.id) = ?"
		args = append(args, len(normalTags))
	}

	if orderByColor != "" {
		query += " ORDER BY " + orderByColor + " LIMIT 50"
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer rows.Close()

	var fileIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan file ID: %w", err)
		}
		fileIDs = append(fileIDs, id)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search rows: %w", err)
	}

	if len(targetColors) > 0 {
		allowedIDs := make(map[int64]bool)
		for _, id := range fileIDs {
			allowedIDs[id] = true
		}
		return SearchImagesByPalette(db, targetColors, 35.0, allowedIDs)
	}

	// Fetch full image records
	var images []Image
	for _, id := range fileIDs {
		img, err := GetImageByID(db, id, false)
		if err != nil {
			fmt.Printf("Warning: failed to load image %d: %v\n", id, err)
			continue
		}
		images = append(images, *img)
	}

	return images, nil
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

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func ExportImagesToAlbum(db *sql.DB, albumName string, imageIDs []int64, galleryDir, albumsBaseDir string) error {
	if len(imageIDs) == 0 {
		return fmt.Errorf("no image IDs provided")
	}

	// Sanitize the album name to prevent directory traversal (e.g., "../../etc")
	cleanAlbumName := filepath.Base(filepath.Clean(albumName))
	if cleanAlbumName == "." || cleanAlbumName == "" {
		return fmt.Errorf("invalid album name")
	}

	// Create the target album directory
	targetDir := filepath.Join(albumsBaseDir, cleanAlbumName)
	if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create album directory: %w", err)
	}

	// Build the IN clause for the query: "SELECT filename FROM files WHERE id IN (?, ?, ?)"
	placeholders := make([]string, len(imageIDs))
	args := make([]interface{}, len(imageIDs))
	for i, id := range imageIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf("SELECT filename FROM files WHERE id IN (%s)", strings.Join(placeholders, ","))

	rows, err := db.Query(query, args...)
	if err != nil {
		return fmt.Errorf("failed to query images for export: %w", err)
	}
	defer rows.Close()

	var filenames []string
	for rows.Next() {
		var fname string
		if err := rows.Scan(&fname); err != nil {
			return fmt.Errorf("failed to scan filename: %w", err)
		}
		filenames = append(filenames, fname)
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("error iterating file rows: %w", err)
	}

	// Copy the physical files
	for _, fname := range filenames {
		srcPath := filepath.Join(galleryDir, fname)
		dstPath := filepath.Join(targetDir, fname)

		err := copyFile(srcPath, dstPath)
		if err != nil {
			fmt.Printf("Warning: failed to copy %s to album %s: %v\n", fname, targetDir, err)
			continue // skip to the next file rather than aborting the whole export
		}
	}

	return nil
}

func GetImagesWithoutMetadata(db *sql.DB) ([]Image, error) {
	query := `
		SELECT id 
		FROM files 
		WHERE active_metadata_id IS NULL AND hasDuplicate IS NULL
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query unmatched images: %w", err)
	}
	defer rows.Close()

	var fileIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan file ID: %w", err)
		}
		fileIDs = append(fileIDs, id)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating unmatched rows: %w", err)
	}

	var images []Image
	for _, id := range fileIDs {
		img, err := GetImageByID(db, id, true)
		if err != nil {
			fmt.Printf("Warning: failed to load unmatched image %d: %v\n", id, err)
			continue
		}
		images = append(images, *img)
	}

	return images, nil
}

// SearchImagesByPalette queries color palettes + weights from SQLite and runs in-memory CIE L*a*b* vibe matching.
// Factors in perceptual color distance Delta E and relative weights, dropping images beyond threshold.
func SearchImagesByPalette(db *sql.DB, targetColors []Color, threshold float64, allowedIDs map[int64]bool) ([]Image, error) {
	if len(targetColors) == 0 {
		return nil, nil
	}

	query := `SELECT file_id, r, g, b, hex, weight FROM image_colors`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query image_colors: %w", err)
	}
	defer rows.Close()

	palettes := make(map[int64][]Color)
	for rows.Next() {
		var fileID int64
		var c Color
		if err := rows.Scan(&fileID, &c.R, &c.G, &c.B, &c.Hex, &c.Weight); err != nil {
			continue
		}
		if allowedIDs != nil && len(allowedIDs) > 0 && !allowedIDs[fileID] {
			continue
		}
		palettes[fileID] = append(palettes[fileID], c)
	}

	type matchScore struct {
		fileID   int64
		distance float64
	}
	var matches []matchScore

	for fileID, imgPalette := range palettes {
		if len(imgPalette) == 0 {
			continue
		}

		distT2P := 0.0
		for _, tc := range targetColors {
			minD := math.MaxFloat64
			for _, pc := range imgPalette {
				d := ColorDistanceLAB(tc, pc)
				if d < minD {
					minD = d
				}
			}
			distT2P += minD
		}
		distT2P /= float64(len(targetColors))

		distP2T := 0.0
		totalWeight := 0.0
		for _, pc := range imgPalette {
			minD := math.MaxFloat64
			for _, tc := range targetColors {
				d := ColorDistanceLAB(pc, tc)
				if d < minD {
					minD = d
				}
			}
			w := pc.Weight
			if w <= 0 {
				w = 1.0 / float64(len(imgPalette))
			}
			distP2T += minD * w
			totalWeight += w
		}
		if totalWeight > 0 {
			distP2T /= totalWeight
		}

		vibeDist := (distT2P + distP2T) / 2.0

		if vibeDist <= threshold {
			matches = append(matches, matchScore{fileID: fileID, distance: vibeDist})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].distance < matches[j].distance
	})

	var images []Image
	for _, m := range matches {
		img, err := GetImageByID(db, m.fileID, false)
		if err != nil || img == nil {
			continue
		}
		images = append(images, *img)
	}

	return images, nil
}

// SearchImagesByColor remains for backward compatibility, delegating to perceptual SearchImagesByPalette.
func SearchImagesByColor(db *sql.DB, targetR, targetG, targetB, threshold int) ([]Image, error) {
	c := Color{
		R:      targetR,
		G:      targetG,
		B:      targetB,
		Hex:    fmt.Sprintf("#%02x%02x%02x", targetR, targetG, targetB),
		Weight: 1.0,
	}
	// Default strict cutoff around 35.0 perceptual Delta E if threshold <= 0
	cut := float64(threshold)
	if cut <= 0 {
		cut = 35.0
	}
	return SearchImagesByPalette(db, []Color{c}, cut, nil)
}

// Brightness ranges from 0 (pitch black) to 255 (pure white).
func SearchImagesByBrightness(db *sql.DB, minBrightness, maxBrightness float64) ([]Image, error) {
	query := `
		SELECT file_id, 
		       AVG((0.299 * r) + (0.587 * g) + (0.114 * b)) as avg_brightness
		FROM image_colors
		GROUP BY file_id
		HAVING avg_brightness >= ? AND avg_brightness <= ?
		ORDER BY avg_brightness ASC
		LIMIT 50
	`

	rows, err := db.Query(query, minBrightness, maxBrightness)
	if err != nil {
		return nil, fmt.Errorf("failed to query brightness: %w", err)
	}
	defer rows.Close()

	var fileIDs []int64
	for rows.Next() {
		var id int64
		var avgBrightness float64
		if err := rows.Scan(&id, &avgBrightness); err != nil {
			return nil, err
		}
		fileIDs = append(fileIDs, id)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating brightness rows: %w", err)
	}

	// Fetch full image records using your existing function
	var images []Image
	for _, id := range fileIDs {
		img, err := GetImageByID(db, id, false)
		if err != nil {
			fmt.Printf("Warning: failed to load image %d: %v\n", id, err)
			continue
		}
		images = append(images, *img)
	}

	return images, nil
}

// ExtractCombinedPaletteFromImages takes a list of file IDs, fetches their extracted colors
// (or extracts on-the-fly if missing), clusters perceptually similar colors using ColorDistanceLAB,
// and guarantees returning at least 5 representative hex colors ranked by dominant weight.
func ExtractCombinedPaletteFromImages(db *sql.DB, fileIDs []int64) ([]string, error) {
	if len(fileIDs) == 0 {
		return []string{}, nil
	}

	type colorEntry struct {
		c      Color
		weight float64
	}
	var allColors []colorEntry

	for _, id := range fileIDs {
		rows, err := db.Query("SELECT r, g, b, hex, weight FROM image_colors WHERE file_id = ?", id)
		if err == nil {
			for rows.Next() {
				var c Color
				var w float64
				if err := rows.Scan(&c.R, &c.G, &c.B, &c.Hex, &w); err == nil {
					allColors = append(allColors, colorEntry{c: c, weight: w})
				}
			}
			rows.Close()
		}

		// If DB didn't provide enough colors, extract more directly from the file
		if len(allColors) < 10 {
			var filename string
			if err := db.QueryRow("SELECT filename FROM files WHERE id = ?", id).Scan(&filename); err == nil {
				filePath := filepath.Join("Gallery", filename)
				if pal, err := ExtractColorPalette(filePath, 10); err == nil {
					for _, c := range pal {
						allColors = append(allColors, colorEntry{c: c, weight: c.Weight})
					}
				}
			}
		}
	}

	if len(allColors) == 0 {
		return []string{"#1e1e2e", "#cba6f7", "#f38ba8", "#a6e3a1", "#89b4fa"}, nil
	}

	// Sort candidate colors by weight descending
	sort.Slice(allColors, func(i, j int) bool {
		return allColors[i].weight > allColors[j].weight
	})

	type cluster struct {
		c      Color
		weight float64
	}
	var clusters []cluster

	// First pass: cluster with moderate perceptual distance
	for _, entry := range allColors {
		merged := false
		for idx, cl := range clusters {
			if ColorDistanceLAB(entry.c, cl.c) < 12.0 {
				clusters[idx].weight += entry.weight
				merged = true
				break
			}
		}
		if !merged {
			clusters = append(clusters, cluster{c: entry.c, weight: entry.weight})
		}
	}

	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].weight > clusters[j].weight
	})

	var result []string
	seenHex := make(map[string]bool)

	for _, cl := range clusters {
		hex := strings.ToLower(cl.c.Hex)
		if !seenHex[hex] {
			seenHex[hex] = true
			result = append(result, cl.c.Hex)
		}
		if len(result) >= 5 {
			break
		}
	}

	// Second pass: if fewer than 5, include remaining unique hexes from allColors
	for _, entry := range allColors {
		if len(result) >= 5 {
			break
		}
		hex := strings.ToLower(entry.c.Hex)
		if !seenHex[hex] {
			seenHex[hex] = true
			result = append(result, entry.c.Hex)
		}
	}

	// Third pass: if STILL fewer than 5 (e.g. single monochromatic image), generate harmonious tints/shades
	if len(result) < 5 && len(allColors) > 0 {
		base := allColors[0].c
		variations := []Color{
			{R: int(math.Min(255, float64(base.R)+float64(255-base.R)*0.35)), G: int(math.Min(255, float64(base.G)+float64(255-base.G)*0.35)), B: int(math.Min(255, float64(base.B)+float64(255-base.B)*0.35))},
			{R: int(float64(base.R) * 0.65), G: int(float64(base.G) * 0.65), B: int(float64(base.B) * 0.65)},
			{R: int(math.Min(255, float64(base.R)+float64(255-base.R)*0.6)), G: int(math.Min(255, float64(base.G)+float64(255-base.G)*0.6)), B: int(math.Min(255, float64(base.B)+float64(255-base.B)*0.6))},
			{R: int(float64(base.R) * 0.4), G: int(float64(base.G) * 0.4), B: int(float64(base.B) * 0.4)},
			{R: base.B, G: base.R, B: base.G}, // subtle rotated accent
		}
		for _, v := range variations {
			if len(result) >= 5 {
				break
			}
			hex := fmt.Sprintf("#%02x%02x%02x", v.R, v.G, v.B)
			if !seenHex[strings.ToLower(hex)] {
				seenHex[strings.ToLower(hex)] = true
				result = append(result, hex)
			}
		}
	}

	return result, nil
}
