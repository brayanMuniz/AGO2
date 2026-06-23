package main

import (
	"database/sql"
	"fmt"
	"log"
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
	`

	_, err := db.Exec(schema)
	return err
}

// OPTIMIZE: If in the future this is too slow use a transaction instead
func ProcessNewUpload(db *sql.DB, apiKey, userName, filename, filePath string) error {
	hash, err := GetPixelHash(filePath)
	if err != nil {
		return fmt.Errorf("failed to hash image: %w", err)
	}

	// Checks for an existing original file (where hasDuplicate is NULL)
	var existingID int64
	var existingFilename string
	err = db.QueryRow("SELECT id, filename FROM files WHERE hash = ? AND hasDuplicate IS NULL", hash).Scan(&existingID, &existingFilename)
	if err == nil {
		fmt.Printf("Duplicate detected! %s already saved as %s\n", filename, existingFilename)

		// Log the duplicate in the database pointing to the original ID
		_, err = db.Exec("INSERT INTO files (filename, hash, hasDuplicate, isFavorite) VALUES (?, ?, ?, FALSE)", filename, hash, existingID)
		if err != nil {
			return fmt.Errorf("failed to insert duplicate file record: %w", err)
		}
		return nil
	}

	// Insert new original file
	_, err = db.Exec("INSERT INTO files (filename, hash, isFavorite) VALUES (?, ?, FALSE)", filename, hash)
	if err != nil {
		return fmt.Errorf("failed to insert file record: %w", err)
	}
	fmt.Printf("Saved new file record: %s\n", filename)

	matches, err := iqdb_upload_request(apiKey, userName, filePath)
	if err != nil {
		return fmt.Errorf("iqdb api failed: %w", err)
	}

	var bestRecordID int64
	var highestScore float64

	for _, match := range matches {
		query := `
			INSERT INTO metadata_records 
			(filename, provider_name, provider_id, score, file_url, large_file_url, rating, source, image_height, image_width, file_size)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		result, err := db.Exec(query,
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

		recordID, _ := result.LastInsertId()

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
			return fmt.Errorf("failed to lock in active metadata: %w", err)
		}
		fmt.Printf("Auto-verified match locked in! (Score: %.2f)\n", highestScore)
	} else {
		fmt.Printf("No 95%%+ match found. Saved %d potential matches to the verification queue.\n", len(matches))
	}

	return nil
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

func GetImageByID(db *sql.DB, fileID int64) (*Image, error) {
	var img Image
	var activeMetadataID sql.NullInt64

	err := db.QueryRow("SELECT filename, hash, active_metadata_id FROM files WHERE id = ?", fileID).
		Scan(&img.FileName, &img.Hash, &activeMetadataID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no file found with ID %d", fileID)
		}
		return nil, fmt.Errorf("failed to fetch file data: %w", err)
	}

	// Fetch all metadata records associated with this file
	query := `
		SELECT 
			id, provider_id, score, file_url, large_file_url, rating, 
			source, image_height, image_width, file_size 
		FROM metadata_records 
		WHERE filename = ?
	`
	rows, err := db.Query(query, img.FileName)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metadata records: %w", err)
	}
	defer rows.Close()

	img.IQDBMatches = make([]IQDBMatch, 0)

	for rows.Next() {
		var recordID int64
		var providerID string
		var score sql.NullFloat64 // Nullable in case older records exist
		var post Post

		var rating, source, fileURL, largeFileURL sql.NullString
		var imgHeight, imgWidth, fileSize sql.NullInt64

		err := rows.Scan(
			&recordID,
			&providerID,
			&score,
			&fileURL,
			&largeFileURL,
			&rating,
			&source,
			&imgHeight,
			&imgWidth,
			&fileSize,
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

		// Populate tags ONLY if this is the active metadata record
		if activeMetadataID.Valid && recordID == activeMetadataID.Int64 {
			err = populateTags(db, recordID, &post)
			if err != nil {
				return nil, fmt.Errorf("failed to populate tags for active record: %w", err)
			}

			mainPost := post // copy the value
			img.MainData = &mainPost
		}

		img.IQDBMatches = append(img.IQDBMatches, IQDBMatch{
			PostID: post.ID,
			Score:  score.Float64,
			Post:   post,
		})
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
