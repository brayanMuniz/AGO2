package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/brayanMuniz/AGO2/internal/models"
	"github.com/brayanMuniz/AGO2/internal/storage"
)

// GET /api/filters
func (a *App) handleGetSavedFilters(w http.ResponseWriter, r *http.Request) {
	filters, err := storage.GetAllSavedFilters(a.DB)
	if err != nil {
		sendJSONError(w, "Failed to fetch saved filters", http.StatusInternalServerError)
		fmt.Printf("Error fetching saved filters: %v\n", err)
		return
	}

	if filters == nil {
		filters = make([]models.SavedFilter, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filters)
}

// POST /api/filters
func (a *App) handleCreateSavedFilter(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		Name      string `json:"name"`
		Query     string `json:"query"`
		SortBy    string `json:"sort_by"`
		SortOrder string `json:"sort_order"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		sendJSONError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(reqBody.Name) == "" {
		sendJSONError(w, "Filter name cannot be empty", http.StatusBadRequest)
		return
	}

	filter, err := storage.CreateSavedFilter(a.DB, strings.TrimSpace(reqBody.Name), reqBody.Query, reqBody.SortBy, reqBody.SortOrder)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			sendJSONError(w, fmt.Sprintf("A filter named '%s' already exists", reqBody.Name), http.StatusConflict)
		} else {
			sendJSONError(w, "Failed to create saved filter", http.StatusInternalServerError)
			fmt.Printf("Error creating saved filter: %v\n", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(filter)
}

// PUT /api/filters/{id}
func (a *App) handleUpdateSavedFilter(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		sendJSONError(w, "Invalid filter ID", http.StatusBadRequest)
		return
	}

	var reqBody struct {
		Name      string `json:"name"`
		Query     string `json:"query"`
		SortBy    string `json:"sort_by"`
		SortOrder string `json:"sort_order"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		sendJSONError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(reqBody.Name) == "" {
		sendJSONError(w, "Filter name cannot be empty", http.StatusBadRequest)
		return
	}

	filter, err := storage.UpdateSavedFilter(a.DB, id, strings.TrimSpace(reqBody.Name), reqBody.Query, reqBody.SortBy, reqBody.SortOrder)
	if err != nil {
		sendJSONError(w, "Failed to update saved filter", http.StatusInternalServerError)
		fmt.Printf("Error updating saved filter %d: %v\n", id, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filter)
}

// DELETE /api/filters/{id}
func (a *App) handleDeleteSavedFilter(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		sendJSONError(w, "Invalid filter ID", http.StatusBadRequest)
		return
	}

	if err := storage.DeleteSavedFilter(a.DB, id); err != nil {
		if strings.Contains(err.Error(), "no saved filter found") {
			sendJSONError(w, err.Error(), http.StatusNotFound)
		} else {
			sendJSONError(w, "Failed to delete saved filter", http.StatusInternalServerError)
			fmt.Printf("Error deleting saved filter %d: %v\n", id, err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Deleted saved filter ID %d", id),
	})
}
