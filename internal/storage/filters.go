package storage

import (
	"database/sql"
	"fmt"

	"github.com/brayanMuniz/AGO2/internal/models"
)

// GetAllSavedFilters returns all saved filter presets ordered by name.
func GetAllSavedFilters(db *sql.DB) ([]models.SavedFilter, error) {
	rows, err := db.Query("SELECT id, name, query, COALESCE(sort_by, 'none'), COALESCE(sort_order, 'desc'), created_at, updated_at FROM saved_filters ORDER BY name ASC")
	if err != nil {
		return nil, fmt.Errorf("failed to query saved filters: %w", err)
	}
	defer rows.Close()

	var filters []models.SavedFilter
	for rows.Next() {
		var f models.SavedFilter
		if err := rows.Scan(&f.ID, &f.Name, &f.Query, &f.SortBy, &f.SortOrder, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan saved filter: %w", err)
		}
		filters = append(filters, f)
	}
	return filters, rows.Err()
}

// CreateSavedFilter inserts a new filter preset and returns it.
func CreateSavedFilter(db *sql.DB, name, query, sortBy, sortOrder string) (*models.SavedFilter, error) {
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

	var f models.SavedFilter
	err = db.QueryRow("SELECT id, name, query, COALESCE(sort_by, 'none'), COALESCE(sort_order, 'desc'), created_at, updated_at FROM saved_filters WHERE id = ?", id).
		Scan(&f.ID, &f.Name, &f.Query, &f.SortBy, &f.SortOrder, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created filter: %w", err)
	}
	return &f, nil
}

// UpdateSavedFilter modifies an existing filter preset and returns the updated record.
func UpdateSavedFilter(db *sql.DB, id int64, name, query, sortBy, sortOrder string) (*models.SavedFilter, error) {
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

	var f models.SavedFilter
	err = db.QueryRow("SELECT id, name, query, COALESCE(sort_by, 'none'), COALESCE(sort_order, 'desc'), created_at, updated_at FROM saved_filters WHERE id = ?", id).
		Scan(&f.ID, &f.Name, &f.Query, &f.SortBy, &f.SortOrder, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated filter: %w", err)
	}
	return &f, nil
}

// DeleteSavedFilter removes a filter preset by ID.
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
