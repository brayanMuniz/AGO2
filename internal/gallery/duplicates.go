package gallery

import (
	"database/sql"
	"fmt"
)

// DuplicateGroup represents a set of files that share the same pixel hash.
type DuplicateGroup struct {
	Hash  string          `json:"hash"`
	Files []DuplicateFile `json:"files"`
}

// DuplicateFile is a single file within a duplicate group.
type DuplicateFile struct {
	ID           int64  `json:"id"`
	FileName     string `json:"file_name"`
	FileSize     int64  `json:"file_size"`
	ImageWidth   int    `json:"image_width"`
	ImageHeight  int    `json:"image_height"`
	IsFavorite   bool   `json:"is_favorite"`
	Organized    bool   `json:"organized"`
	HasDuplicate *int64 `json:"has_duplicate,omitempty"`
}

// DuplicateScanResult is the JSON response for a duplicate scan.
type DuplicateScanResult struct {
	Status        string           `json:"status"` // "completed", "processing", "failed"
	TotalScanned  int              `json:"total_scanned"`
	DuplicateGroups []DuplicateGroup `json:"duplicate_groups"`
	NewlyMarked   int              `json:"newly_marked"`
	Error         string           `json:"error,omitempty"`
}

// FindDuplicates scans the database for files sharing the same pixel hash.
// It finds groups where multiple files have identical hashes, including files
// that were previously missed because they had different filenames but the
// same content. Newly discovered duplicates get their hasDuplicate field set.
func FindDuplicates(db *sql.DB) (*DuplicateScanResult, error) {
	result := &DuplicateScanResult{
		Status:          "completed",
		DuplicateGroups: []DuplicateGroup{},
	}

	// Count total files for progress context
	var totalFiles int
	if err := db.QueryRow("SELECT COUNT(*) FROM files").Scan(&totalFiles); err != nil {
		return nil, fmt.Errorf("failed to count files: %w", err)
	}
	result.TotalScanned = totalFiles

	// Find all hashes that appear in more than one file
	rows, err := db.Query(`
		SELECT hash, COUNT(*) as cnt
		FROM files
		WHERE hash != ''
		GROUP BY hash
		HAVING cnt > 1
		ORDER BY cnt DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query duplicate hashes: %w", err)
	}
	defer rows.Close()

	var dupHashes []string
	for rows.Next() {
		var hash string
		var cnt int
		if err := rows.Scan(&hash, &cnt); err != nil {
			continue
		}
		dupHashes = append(dupHashes, hash)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating duplicate hashes: %w", err)
	}

	if len(dupHashes) == 0 {
		return result, nil
	}

	// For each duplicate hash, fetch all files in the group
	newlyMarked := 0

	for _, hash := range dupHashes {
		fileRows, err := db.Query(`
			SELECT id, filename, file_size, image_width, image_height, isFavorite, organized, hasDuplicate
			FROM files
			WHERE hash = ?
			ORDER BY id ASC
		`, hash)
		if err != nil {
			fmt.Printf("Warning: failed to query files for hash %s: %v\n", hash[:8], err)
			continue
		}

		var group DuplicateGroup
		group.Hash = hash
		group.Files = []DuplicateFile{}

		var originalID int64
		allAlreadyMarked := true

		for fileRows.Next() {
			var f DuplicateFile
			var hasDup sql.NullInt64

			if err := fileRows.Scan(&f.ID, &f.FileName, &f.FileSize, &f.ImageWidth, &f.ImageHeight, &f.IsFavorite, &f.Organized, &hasDup); err != nil {
				continue
			}

			if hasDup.Valid {
				val := hasDup.Int64
				f.HasDuplicate = &val
			} else {
				allAlreadyMarked = false
			}

			// The first file without hasDuplicate set is the "original"
			if originalID == 0 && !hasDup.Valid {
				originalID = f.ID
			}

			group.Files = append(group.Files, f)
		}
		fileRows.Close()

		if len(group.Files) < 2 {
			continue
		}

		// Mark newly discovered duplicates: any file in the group (except the original)
		// that doesn't already have hasDuplicate set
		if !allAlreadyMarked && originalID > 0 {
			for _, f := range group.Files {
				if f.ID != originalID && f.HasDuplicate == nil {
					_, err := db.Exec("UPDATE files SET hasDuplicate = ? WHERE id = ?", originalID, f.ID)
					if err != nil {
						fmt.Printf("Warning: failed to mark file %d as duplicate of %d: %v\n", f.ID, originalID, err)
						continue
					}
					newlyMarked++
					fmt.Printf("Marked duplicate: %s (ID %d) -> original %d\n", f.FileName, f.ID, originalID)
				}
			}
		}

		result.DuplicateGroups = append(result.DuplicateGroups, group)
	}

	result.NewlyMarked = newlyMarked

	return result, nil
}
