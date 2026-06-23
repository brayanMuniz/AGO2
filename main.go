package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	database, err := InitDB("./gallery.db")
	if err != nil {
		fmt.Printf("", err)
	}
	defer database.Close()

	app := &App{
		DB: database,
	}

	http.HandleFunc("/api/image", app.handleGetImage)
	http.HandleFunc("/api/search", app.handleSearch)

	port := ":8080"
	fmt.Printf("Server is running on http://localhost%s\n", port)

	err = http.ListenAndServe(port, nil)
	if err != nil {
		fmt.Println("Server failed to start: %v\n", err)
		return
	}

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

	// err = ProcessGalleryDirectory(database, apiKey, userName, "./Gallery/")
	// if err != nil {
	// 	fmt.Printf("", err)
	// }
}

// OPTIMIZE: Add Go routines
func ProcessGalleryDirectory(db *sql.DB, apikey, userName, dirPath string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		ext := strings.ToLower(filepath.Ext(fileName))

		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
			continue
		}

		filePath := filepath.Join(dirPath, fileName)

		err := ProcessNewUpload(db, apikey, userName, fileName, filePath)
		if err != nil {
			return err
		}

		time.Sleep(1 * time.Second) // for rate limits
	}

	return nil
}
