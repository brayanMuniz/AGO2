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
		filename TEXT PRIMARY KEY,
		hash TEXT NOT NULL UNIQUE,
		active_metadata_id INTEGER,
		
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (active_metadata_id) REFERENCES metadata_records (id) ON DELETE SET NULL
	);

	-- TRACKS THE METADATA SOURCE RECORDS
	CREATE TABLE IF NOT EXISTS metadata_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		filename TEXT NOT NULL,
		
		-- THIRD PARTY TRACKING
		provider_name TEXT NOT NULL,
		provider_id TEXT,
		
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

func ProcessNewUpload(db *sql.DB, apiKey, userName, filename, filePath string) error {
	hash, err := GetPixelHash(filePath)
	if err != nil {
		return fmt.Errorf("failed to hash image: %w", err)
	}

	// Returns early if duplicate hash
	var existingFilename string
	err = db.QueryRow("SELECT filename FROM files WHERE hash = ?", hash).Scan(&existingFilename)
	if err == nil {
		fmt.Printf("Duplicate detected! %s already saved as %s\n", filename, existingFilename)
		return nil
	}

	_, err = db.Exec("INSERT INTO files (filename, hash) VALUES (?, ?)", filename, hash)
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
			(filename, provider_name, provider_id, file_url, large_file_url, rating, source, image_height, image_width, file_size) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		result, err := db.Exec(query,
			filename,
			"danbooru",                       // provider_name
			fmt.Sprintf("%d", match.Post.ID), // provider_id
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

// Pass 0 for the limit in order to get all of them
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
