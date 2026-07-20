package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brayanMuniz/AGO2/internal/models"
)

// GetImageByID fetches a full image record including its active metadata.
func GetImageByID(db *sql.DB, fileID int64, includeMatches bool) (*models.Image, error) {
	var img models.Image
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
		var post models.Post

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

// populateTags fetches tags for a specific metadata record and organizes them into the Post struct.
func populateTags(db *sql.DB, metadataID int64, post *models.Post) error {
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

// DeleteImageByID removes an image record from the database and deletes its files from disk.
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

// UpdateImage applies partial updates to an image record.
func UpdateImage(db *sql.DB, fileID int64, params models.UpdateImageParams) error {
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
		SaveTags(db, newRecordID, post.TagsArtist, "artist")
		SaveTags(db, newRecordID, post.TagsCharacters, "character")
		SaveTags(db, newRecordID, post.TagsCopyright, "copyright")
		SaveTags(db, newRecordID, post.TagsGeneral, "general")
		SaveTags(db, newRecordID, post.TagsMeta, "meta")

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

// SaveTags inserts tags into the dictionary and links them to a metadata record.
func SaveTags(db *sql.DB, recordID int64, tags []string, category string) {
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

// GetApprovedMetadata retrieves the active metadata for a file by filename.
func GetApprovedMetadata(db *sql.DB, filename string) (*models.Post, error) {
	query := `
		SELECT 
			m.provider_id, m.file_url, m.large_file_url, m.rating, 
			m.source, f.image_height, f.image_width, f.file_size
		FROM files f
		JOIN metadata_records m ON f.active_metadata_id = m.id
		WHERE f.filename = ?
	`

	var post models.Post
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
