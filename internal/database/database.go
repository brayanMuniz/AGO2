package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

// InitDB opens (or creates) the SQLite database at dbPath, creates all required
// tables and indexes, and seeds default data on first run.
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

	-- APP SETTINGS
	CREATE TABLE IF NOT EXISTS app_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Index for faster lookups when we doing math on colors columns
	CREATE INDEX IF NOT EXISTS idx_image_colors_rgb ON image_colors(r, g, b);
	CREATE INDEX IF NOT EXISTS idx_image_colors_file_id ON image_colors(file_id);

	-- Speeds up "Missing Data", "Duplicate", and "Organized" queries
	CREATE INDEX IF NOT EXISTS idx_files_status ON files(active_metadata_id, hasDuplicate, organized);
	CREATE INDEX IF NOT EXISTS idx_files_favorite ON files(isFavorite);
	CREATE INDEX IF NOT EXISTS idx_files_created_at ON files(created_at);
	CREATE INDEX IF NOT EXISTS idx_files_hash ON files(hash);

	-- Speeds up tag searches and reverse join lookups
	CREATE INDEX IF NOT EXISTS idx_record_tags_tag_id ON record_tags(tag_id);
	CREATE INDEX IF NOT EXISTS idx_metadata_records_filename ON metadata_records(filename);
	`
	_, err := db.Exec(schema)
	return err
}
