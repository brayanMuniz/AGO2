package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/brayanMuniz/AGO2/internal/gallery"
	"github.com/brayanMuniz/AGO2/internal/models"
	"github.com/brayanMuniz/AGO2/internal/storage"
)

// GET /api/palettes
func (a *App) handleGetSavedPalettes(w http.ResponseWriter, r *http.Request) {
	palettes, err := storage.GetSavedPalettes(a.DB)
	if err != nil {
		sendJSONError(w, "Failed to retrieve saved palettes", http.StatusInternalServerError)
		return
	}
	if palettes == nil {
		palettes = make([]models.SavedPalette, 0)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(palettes)
}

// POST /api/palettes
func (a *App) handleCreateSavedPalette(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		Name   string   `json:"name"`
		Colors []string `json:"colors"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		sendJSONError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(reqBody.Name) == "" {
		sendJSONError(w, "Palette name cannot be empty", http.StatusBadRequest)
		return
	}
	if len(reqBody.Colors) == 0 {
		sendJSONError(w, "Palette must contain at least one color", http.StatusBadRequest)
		return
	}

	palette, err := storage.CreateSavedPalette(a.DB, strings.TrimSpace(reqBody.Name), reqBody.Colors)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			sendJSONError(w, fmt.Sprintf("A palette named '%s' already exists", reqBody.Name), http.StatusConflict)
		} else {
			sendJSONError(w, "Failed to save palette", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(palette)
}

// DELETE /api/palettes/{id}
func (a *App) handleDeleteSavedPalette(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		sendJSONError(w, "Invalid palette ID", http.StatusBadRequest)
		return
	}

	if err := storage.DeleteSavedPalette(a.DB, id); err != nil {
		sendJSONError(w, "Failed to delete saved palette", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Deleted saved palette ID %d", id),
	})
}

// POST /api/palettes/extract
func (a *App) handleExtractPaletteFromImages(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil || len(reqBody.IDs) == 0 {
		sendJSONError(w, "Invalid or empty IDs list", http.StatusBadRequest)
		return
	}

	colors, err := gallery.ExtractCombinedPaletteFromImages(a.DB, reqBody.IDs)
	if err != nil {
		sendJSONError(w, "Failed to extract palette from selected images", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{
		"colors": colors,
	})
}
