package main

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

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
