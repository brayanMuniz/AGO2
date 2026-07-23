package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/brayanMuniz/AGO2/internal/config"
)

// App holds shared dependencies for all HTTP handlers.
type App struct {
	DB  *sql.DB
	Cfg *config.Config
}

// sendJSONError writes a JSON-formatted error response.
func sendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// noCacheHandler wraps a handler to prevent aggressive browser caching on replaced images and thumbnails.
func noCacheHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		h.ServeHTTP(w, r)
	})
}

// RegisterRoutes sets up all HTTP routes on the provided ServeMux.
func RegisterRoutes(mux *http.ServeMux, app *App) {
	// Static file serving for images and thumbnails
	mux.Handle("/images/", noCacheHandler(http.StripPrefix("/images/", http.FileServer(http.Dir(app.Cfg.GalleryDir)))))
	mux.Handle("/thumbnails/", noCacheHandler(http.StripPrefix("/thumbnails/", http.FileServer(http.Dir(app.Cfg.ThumbnailDir)))))

	// Image endpoints
	mux.HandleFunc("GET /api/image/{id}", app.handleGetImage)
	mux.HandleFunc("GET /api/image/{id}/matches", app.handleGetImageMatches)
	mux.HandleFunc("PATCH /api/image/{id}", app.handleImageUpdate)
	mux.HandleFunc("DELETE /api/image/{id}", app.handleDeleteImage)
	mux.HandleFunc("POST /api/image/batch-delete", app.handleBatchDeleteImages)
	mux.HandleFunc("POST /api/image/download-match", app.handleDownloadMatch)

	// Search & autocomplete
	mux.HandleFunc("GET /api/search", app.handleSearch)
	mux.HandleFunc("GET /api/tags/autocomplete", app.handleTagAutocomplete)

	// Gallery processing
	mux.HandleFunc("POST /api/process-gallery", app.handleProcessGallery)
	mux.HandleFunc("GET /api/process-gallery/status", app.handleGetJobStatus)
	mux.HandleFunc("POST /api/find-duplicates", app.handleFindDuplicates)

	// Filters
	mux.HandleFunc("GET /api/filters", app.handleGetSavedFilters)
	mux.HandleFunc("POST /api/filters", app.handleCreateSavedFilter)
	mux.HandleFunc("PUT /api/filters/{id}", app.handleUpdateSavedFilter)
	mux.HandleFunc("DELETE /api/filters/{id}", app.handleDeleteSavedFilter)

	// Palettes
	mux.HandleFunc("GET /api/palettes", app.handleGetSavedPalettes)
	mux.HandleFunc("POST /api/palettes", app.handleCreateSavedPalette)
	mux.HandleFunc("DELETE /api/palettes/{id}", app.handleDeleteSavedPalette)
	mux.HandleFunc("POST /api/palettes/extract", app.handleExtractPaletteFromImages)

	// Albums
	mux.HandleFunc("POST /api/album/export", app.handleExportAlbum)

	// Stats
	mux.HandleFunc("GET /api/stats", app.handleGetStats)

	// Settings
	mux.HandleFunc("GET /api/settings/danbooru", app.handleGetDanbooruSettings)
	mux.HandleFunc("POST /api/settings/danbooru", app.handleSaveDanbooruSettings)

	// Proxy
	mux.HandleFunc("GET /api/proxy-image", proxyImageHandler)
}
