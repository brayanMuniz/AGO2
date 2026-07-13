package main

import (
	"database/sql"
	"fmt"
)

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
