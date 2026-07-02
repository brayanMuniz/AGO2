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

	// reroute images for frontend
	http.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("./Gallery"))))
	http.Handle("/thumbnails/", http.StripPrefix("/thumbnails/", http.FileServer(http.Dir("./thumbnails"))))

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
	http.HandleFunc("GET /api/images/unmatched", app.handleGetUnmatchedImages)
	http.HandleFunc("GET /api/search", app.handleSearch)
	http.HandleFunc("GET /api/process-gallery/status", app.handleGetJobStatus)

	http.HandleFunc("POST /api/process-gallery", app.handleProcessGallery)
	http.HandleFunc("POST /api/album/export", app.handleExportAlbum)

	http.HandleFunc("PATCH /api/image/{id}", app.handleImageUpdate)

	http.HandleFunc("DELETE /api/image/{id}", app.handleDeleteImage)

	port := ":8080"
	fmt.Printf("Server is running on http://localhost%s\n", port)

	err = http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalf("Server failed to start: %v\n", err)
	}
}
