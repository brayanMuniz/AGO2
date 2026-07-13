package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	database, err := InitDB("./gallery.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Helper to prevent aggressive browser caching on replaced images and thumbnails
	noCacheHandler := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			h.ServeHTTP(w, r)
		})
	}

	// reroute images for frontend
	http.Handle("/images/", noCacheHandler(http.StripPrefix("/images/", http.FileServer(http.Dir("./Gallery")))))
	http.Handle("/thumbnails/", noCacheHandler(http.StripPrefix("/thumbnails/", http.FileServer(http.Dir("./thumbnails")))))

	err = godotenv.Load("./.env")
	if err != nil {
		log.Fatalf("Could not read the .env file", err)
	}

	userName := os.Getenv("USERNAME")
	if userName == "" {
		fmt.Println("DANBOORU userName is empty")
		return
	}

	apiKey := os.Getenv("DANBOORU_KEY")
	if apiKey == "" {
		fmt.Println("DANBOORU api key is empty")
		return
	}

	app := &App{
		DB: database,
	}

	http.HandleFunc("GET /api/image/{id}", app.handleGetImage)
	http.HandleFunc("GET /api/image/{id}/matches", app.handleGetImageMatches)
	http.HandleFunc("GET /api/search", app.handleSearch)
	http.HandleFunc("GET /api/process-gallery/status", app.handleGetJobStatus)
	http.HandleFunc("GET /api/proxy-image", ProxyImageHandler)
	http.HandleFunc("GET /api/tags/autocomplete", app.handleTagAutocomplete)
	http.HandleFunc("GET /api/filters", app.handleGetSavedFilters)
	http.HandleFunc("GET /api/palettes", app.handleGetSavedPalettes)
	http.HandleFunc("GET /api/stats", app.handleGetStats)

	http.HandleFunc("POST /api/process-gallery", app.handleProcessGallery)
	http.HandleFunc("POST /api/album/export", app.handleExportAlbum)
	http.HandleFunc("POST /api/image/batch-delete", app.handleBatchDeleteImages)
	http.HandleFunc("POST /api/filters", app.handleCreateSavedFilter)
	http.HandleFunc("POST /api/palettes", app.handleCreateSavedPalette)
	http.HandleFunc("POST /api/palettes/extract", app.handleExtractPaletteFromImages)
	http.HandleFunc("POST /api/image/download-match", app.handleDownloadMatch)

	http.HandleFunc("PATCH /api/image/{id}", app.handleImageUpdate)

	http.HandleFunc("PUT /api/filters/{id}", app.handleUpdateSavedFilter)

	http.HandleFunc("DELETE /api/image/{id}", app.handleDeleteImage)
	http.HandleFunc("DELETE /api/filters/{id}", app.handleDeleteSavedFilter)
	http.HandleFunc("DELETE /api/palettes/{id}", app.handleDeleteSavedPalette)

	port := ":8080"
	fmt.Printf("Server is running on http://localhost%s\n", port)

	err = http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalf("Server failed to start: %v\n", err)
	}
}
