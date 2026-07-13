package main

import (
	"database/sql"
	"fmt"
	"strings"
)

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
