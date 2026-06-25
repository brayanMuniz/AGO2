package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	database, err := InitDB("./gallery.db")
	if err != nil {
		fmt.Printf("", err)
	}
	defer database.Close()

	// reroute images for frontend
	http.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("./Gallery"))))
	http.Handle("/thumbnails/", http.StripPrefix("/thumbnails/", http.FileServer(http.Dir("./thumbnails"))))

	err = godotenv.Load("./.env")
	if err != nil {
		fmt.Println("Could not read the .env file")
		return
	}

	userName := os.Getenv("USERNAME")
	if userName == "" {
		fmt.Println("userName is empty")
		return
	}

	apiKey := os.Getenv("DANBOORU_KEY")
	if apiKey == "" {
		fmt.Println("api key is empty")
		return
	}

	app := &App{
		DB: database,
	}

	http.HandleFunc("GET /api/image/{id}", app.handleGetImage)
	http.HandleFunc("GET /api/search", app.handleSearch)

	http.HandleFunc("POST /api/process-gallery", app.handleProcessGallery)
	http.HandleFunc("POST /api/album/export", app.handleExportAlbum)

	http.HandleFunc("PATCH /api/image/{id}/favorite", app.handleUpdateFavorite)

	http.HandleFunc("DELETE /api/image/{id}", app.handleDeleteImage)

	port := ":8080"
	fmt.Printf("Server is running on http://localhost%s\n", port)

	err = http.ListenAndServe(port, nil)
	if err != nil {
		fmt.Println("Server failed to start: %v\n", err)
		return
	}
}
