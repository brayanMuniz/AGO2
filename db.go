package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
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

	err = createTables(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
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
		thumbnail_path TEXT DEFAULT NULL,
		
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
		image_height INTEGER,
		image_width INTEGER,
		file_size INTEGER,
		
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
	    FOREIGN KEY (file_id) REFERENCES files (id) ON DELETE CASCADE
	);

	-- Index for faster lookups when we doing math on colors columns
	CREATE INDEX IF NOT EXISTS idx_image_colors_rgb ON image_colors(r, g, b);

	-- Speeds up "Missing Data" and "Duplicate" queries
	CREATE INDEX IF NOT EXISTS idx_files_status ON files(active_metadata_id, hasDuplicate);
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

	palette, err := ExtractColorPalette(filePath, 5) // Get top 5 colors
	if err != nil {
		fmt.Printf("Warning: failed to extract color palette for %s: %v\n", filename, err)
		palette = []Color{} // Default to empty array on failure
	}

	// Checks for an existing original file
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
		_, err = db.Exec("INSERT INTO files (filename, hash, hasDuplicate, isFavorite) VALUES (?, ?, ?, FALSE)", filename, hash, existingID)
		if err != nil {
			return result, fmt.Errorf("failed to insert duplicate file record: %w", err)
		}

		result.Skipped = true
		return result, nil
	}

	// Insert new original file
	execRes, err := db.Exec("INSERT INTO files (filename, hash, isFavorite) VALUES (?, ?, FALSE)", filename, hash)
	if err != nil {
		return result, fmt.Errorf("failed to insert file record: %w", err)
	}
	newFileID, _ := execRes.LastInsertId()
	fmt.Printf("Saved new file record: %s\n", filename)

	// Save the extracted colors to the linked table
	for _, color := range palette {
		_, err = db.Exec(
			"INSERT INTO image_colors (file_id, r, g, b, hex) VALUES (?, ?, ?, ?, ?)",
			newFileID, color.R, color.G, color.B, color.Hex,
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

	var bestRecordID int64
	var highestScore float64
	for _, match := range matches {

		// Do not add useless data
		if match.Score < 69.0 {
			continue
		}

		query := `
			INSERT INTO metadata_records 
			(filename, provider_name, provider_id, score, file_url, large_file_url, rating, source, image_height, image_width, file_size)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		execResMatch, err := db.Exec(query,
			filename,
			"danbooru",
			fmt.Sprintf("%d", match.Post.ID),
			match.Score,
			match.Post.FileURL,
			match.Post.LargeFileURL,
			match.Post.Rating,
			match.Post.Source,
			match.Post.ImageHeight,
			match.Post.ImageWidth,
			match.Post.FileSize,
		)
		if err != nil {
			fmt.Printf("Error saving match record for %d: %v\n", match.Post.ID, err)
			continue
		}

		recordID, _ := execResMatch.LastInsertId()

		saveTags(db, recordID, match.Post.TagsArtist, "artist")
		saveTags(db, recordID, match.Post.TagsCharacters, "character")
		saveTags(db, recordID, match.Post.TagsCopyright, "copyright")
		saveTags(db, recordID, match.Post.TagsGeneral, "general")
		saveTags(db, recordID, match.Post.TagsMeta, "meta")

		if match.Score > highestScore {
			highestScore = match.Score
			bestRecordID = recordID
		}
	}

	// Auto-Verify if 95%+ match
	if highestScore >= 95.0 && bestRecordID > 0 {
		_, err = db.Exec("UPDATE files SET active_metadata_id = ? WHERE filename = ?", bestRecordID, filename)
		if err != nil {
			return result, fmt.Errorf("failed to lock in active metadata: %w", err)
		}
		result.AutoMatch = true
	} else {
		fmt.Printf("No 95%%+ match found. Saved %d potential matches to the verification queue.\n", len(matches))
	}

	return result, nil
}

func GetApprovedMetadata(db *sql.DB, filename string) (*Post, error) {
	query := `
		SELECT 
			m.provider_id, m.file_url, m.large_file_url, m.rating, 
			m.source, m.image_height, m.image_width, m.file_size
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

	err := db.QueryRow("SELECT filename, hash, isFavorite, active_metadata_id, IFNULL(thumbnail_path, ''), hasDuplicate FROM files WHERE id = ?", fileID).
		Scan(&img.FileName, &img.Hash, &img.IsFavorite, &activeMetadataID, &img.ThumbnailPath, &hasDuplicate)
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

	var rows *sql.Rows
	query := `
	    SELECT id, provider_id, score, file_url, large_file_url, rating, 
		   source, image_height, image_width, file_size 
	    FROM metadata_records 
	    WHERE `
	var queryArg any

	if includeMatches {
		query += "filename = ?"
		queryArg = img.FileName
		img.IQDBMatches = make([]IQDBMatch, 0) // Initialize as [] so it isn't null
	} else {
		if !activeMetadataID.Valid {
			return &img, nil
		}
		query += "id = ?"
		queryArg = activeMetadataID.Int64
	}
	rows, err = db.Query(query, queryArg)

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
		var imgHeight, imgWidth, fileSize sql.NullInt64

		err := rows.Scan(
			&recordID, &providerID, &score, &fileURL, &largeFileURL, &rating,
			&source, &imgHeight, &imgWidth, &fileSize,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan metadata row: %w", err)
		}

		fmt.Sscanf(providerID, "%d", &post.ID)
		post.FileURL = fileURL.String
		post.LargeFileURL = largeFileURL.String
		post.Rating = rating.String
		post.Source = source.String
		post.ImageHeight = int(imgHeight.Int64)
		post.ImageWidth = int(imgWidth.Int64)
		post.FileSize = int(fileSize.Int64)

		if (activeMetadataID.Valid && recordID == activeMetadataID.Int64) || includeMatches {
			err = populateTags(db, recordID, &post)
			if err != nil {
				return nil, fmt.Errorf("failed to populate tags for record: %w", err)
			}
		}

		if activeMetadataID.Valid && recordID == activeMetadataID.Int64 {
			mainPost := post
			img.MainData = &mainPost
		}

		if includeMatches {
			img.IQDBMatches = append(img.IQDBMatches, IQDBMatch{
				PostID: post.ID,
				Score:  score.Float64,
				Post:   post,
			})
		}
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
	var ratings []string
	var isFavoriteFilter *bool
	var orderByColor string

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

		whereClauses = append(whereClauses, fmt.Sprintf("m.%s %s ?", column, op))
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

		} else if strings.HasPrefix(lowerToken, "color:#") {
			hexStr := strings.TrimPrefix(lowerToken, "color:")
			var r, g, b int
			fmt.Sscanf(hexStr, "#%02x%02x%02x", &r, &g, &b)

			// SORT by the closest color distance using a subquery
			orderByColor = fmt.Sprintf(`
				(SELECT MIN((r - %d)*(r - %d) + (g - %d)*(g - %d) + (b - %d)*(b - %d)) 
				 FROM image_colors WHERE file_id = f.id) ASC
			`, r, r, g, g, b, b)

		} else if lowerToken == "is:missing" {
			whereClauses = append(whereClauses, "f.active_metadata_id IS NULL AND f.hasDuplicate IS NULL")
		} else if lowerToken == "is:duplicate" {
			whereClauses = append(whereClauses, "f.hasDuplicate IS NOT NULL")
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

		query := `
			INSERT INTO metadata_records 
			(filename, provider_name, provider_id, score, file_url, large_file_url, rating, source, image_height, image_width, file_size)
			VALUES (?, 'danbooru', ?, 100.0, ?, ?, ?, ?, ?, ?, ?)
		`

		execRes, err := db.Exec(query,
			filename,
			fmt.Sprintf("%d", post.ID), // The Danbooru ID
			post.FileURL,
			post.LargeFileURL,
			post.Rating,
			post.Source,
			post.ImageHeight,
			post.ImageWidth,
			post.FileSize,
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

		// Tell the files table to use this new internal ID
		setClauses = append(setClauses, "active_metadata_id = ?")
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

// threshold determines how "strict" the match is (e.g., 2000 is very strict, 10000 is loose).
func SearchImagesByColor(db *sql.DB, targetR, targetG, targetB, threshold int) ([]Image, error) {
	// calculate the squared Euclidean distance directly in SQL.
	query := `
		SELECT DISTINCT file_id, 
		       ((r - ?) * (r - ?) + (g - ?) * (g - ?) + (b - ?) * (b - ?)) as distance
		FROM image_colors
		WHERE distance <= ?
		ORDER BY distance ASC
		LIMIT 50
	`

	// Pass the target RGB variables twice each to satisfy the math, plus the threshold
	rows, err := db.Query(query, targetR, targetR, targetG, targetG, targetB, targetB, threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to query colors: %w", err)
	}
	defer rows.Close()

	var fileIDs []int64
	for rows.Next() {
		var id int64
		var distance int
		if err := rows.Scan(&id, &distance); err != nil {
			return nil, err
		}
		fileIDs = append(fileIDs, id)
	}

	var images []Image
	for _, id := range fileIDs {
		img, err := GetImageByID(db, id, false)
		if err != nil {
			continue
		}
		images = append(images, *img)
	}

	return images, nil
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
