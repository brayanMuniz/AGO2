package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type App struct {
	DB *sql.DB
}

// Helper to send JSON formatted error messages
func sendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// /api/image?id=X
func (a *App) handleGetImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		sendJSONError(w, "Missing 'id' query parameter", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		sendJSONError(w, "Invalid 'id' parameter; must be an integer", http.StatusBadRequest)
		return
	}

	img, err := GetImageByID(a.DB, id, true)
	if err != nil {
		// Differentiate between a 404 Not Found and a 500 Server Error
		if strings.Contains(err.Error(), "no file found") {
			sendJSONError(w, err.Error(), http.StatusNotFound)
		} else {
			sendJSONError(w, "Failed to fetch image", http.StatusInternalServerError)
			fmt.Printf("Database error fetching image %d: %v\n", id, err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(img)
}

// /api/search?tags=tag1+tag2
func (a *App) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tagsParam := r.URL.Query().Get("tags")
	if tagsParam == "" {
		sendJSONError(w, "Missing 'tags' query parameter", http.StatusBadRequest)
		return
	}

	// "?tags=tag1+tag2" becomes "tag1 tag2".
	tags := strings.Fields(tagsParam)

	images, err := SearchImagesByTags(a.DB, tags)
	if err != nil {
		sendJSONError(w, "Failed to search images", http.StatusInternalServerError)
		fmt.Printf("Database error searching tags %v: %v\n", tags, err)
		return
	}

	// If no images are found, return an empty array instead of null
	if images == nil {
		images = make([]Image, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(images)
}
