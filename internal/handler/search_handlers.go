package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/brayanMuniz/AGO2/internal/gallery"
	"github.com/brayanMuniz/AGO2/internal/models"
)

var (
	tagToCategoryList []models.TagSuggestion
	tagToCategoryOnce sync.Once
)

func loadTagToCategoryJSON(jsonPath string) {
	tagToCategoryOnce.Do(func() {
		fileBytes, err := os.ReadFile(jsonPath)
		if err != nil {
			fmt.Printf("Warning: failed to read %s: %v\n", jsonPath, err)
			return
		}
		var rawMap map[string]string
		if err := json.Unmarshal(fileBytes, &rawMap); err != nil {
			fmt.Printf("Warning: failed to unmarshal %s: %v\n", jsonPath, err)
			return
		}
		for k, v := range rawMap {
			tagToCategoryList = append(tagToCategoryList, models.TagSuggestion{
				Name:     k,
				Category: v,
			})
		}
		fmt.Printf("Loaded %d tags from %s\n", len(tagToCategoryList), jsonPath)
	})
}

// GET /api/search?tags=tag1+tag2&limit=50&offset=0&sort_by=id&sort_order=desc
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

	limit := 50
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil {
			limit = l
		}
	}

	offset := 0
	if oStr := r.URL.Query().Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			offset = o
		}
	}

	sortBy := r.URL.Query().Get("sort_by")
	sortOrder := r.URL.Query().Get("sort_order")

	// "?tags=tag1+tag2" becomes "tag1 tag2".
	tags := strings.Fields(tagsParam)

	images, totalCount, err := gallery.SearchImages(a.DB, tags, limit, offset, sortBy, sortOrder)
	if err != nil {
		sendJSONError(w, "Failed to search images", http.StatusInternalServerError)
		fmt.Printf("Database error searching tags %v: %v\n", tags, err)
		return
	}

	// If no images are found, return an empty array instead of null
	if images == nil {
		images = make([]models.Image, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"images":      images,
		"total_count": totalCount,
	})
}

// GET /api/tags/autocomplete?query=...&category=...
func (a *App) handleTagAutocomplete(w http.ResponseWriter, r *http.Request) {
	loadTagToCategoryJSON(a.Cfg.TagCategoryJSONPath)

	queryStr := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query")))
	categoryFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("category")))

	// Check if query starts with a category prefix (e.g. "artist:miku")
	if colonIdx := strings.Index(queryStr, ":"); colonIdx > 0 && categoryFilter == "" {
		prefix := queryStr[:colonIdx]
		catMap := map[string]string{
			"artist":    "artist",
			"character": "character",
			"copyright": "copyright",
			"general":   "general",
			"meta":      "meta",
			"rating":    "rating",
			"year":      "year",
		}
		if cat, ok := catMap[prefix]; ok {
			categoryFilter = cat
			queryStr = strings.TrimSpace(queryStr[colonIdx+1:])
		}
	}

	prefixMatches := []models.TagSuggestion{}
	containsMatches := []models.TagSuggestion{}

	for _, item := range tagToCategoryList {
		if categoryFilter != "" && !strings.EqualFold(item.Category, categoryFilter) {
			continue
		}
		if queryStr == "" {
			prefixMatches = append(prefixMatches, item)
			if len(prefixMatches) >= 30 {
				break
			}
			continue
		}

		lowerName := strings.ToLower(item.Name)
		if strings.HasPrefix(lowerName, queryStr) {
			prefixMatches = append(prefixMatches, item)
		} else if strings.Contains(lowerName, queryStr) {
			containsMatches = append(containsMatches, item)
		}
	}

	suggestions := append(prefixMatches, containsMatches...)

	// DB fallback: find tags in the database that weren't in the JSON file
	if queryStr != "" && len(suggestions) < 30 {
		// Build a set of already-found tag names for deduplication
		foundNames := make(map[string]bool)
		for _, s := range suggestions {
			foundNames[strings.ToLower(s.Name)] = true
		}

		dbQuery := `SELECT name, category FROM tags WHERE LOWER(name) LIKE ?`
		dbArgs := []interface{}{"%" + queryStr + "%"}

		if categoryFilter != "" {
			dbQuery += ` AND LOWER(category) = ?`
			dbArgs = append(dbArgs, categoryFilter)
		}

		dbQuery += ` ORDER BY name LIMIT ?`
		remaining := 30 - len(suggestions)
		dbArgs = append(dbArgs, remaining+10) // fetch a few extra to account for dedup filtering

		rows, err := a.DB.Query(dbQuery, dbArgs...)
		if err == nil {
			defer rows.Close()
			for rows.Next() && len(suggestions) < 30 {
				var name, category string
				if err := rows.Scan(&name, &category); err != nil {
					continue
				}
				if foundNames[strings.ToLower(name)] {
					continue
				}
				foundNames[strings.ToLower(name)] = true
				suggestions = append(suggestions, models.TagSuggestion{
					Name:     name,
					Category: category,
				})
			}
		}
	}

	if len(suggestions) > 30 {
		suggestions = suggestions[:30]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(suggestions)
}
