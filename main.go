package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/joho/godotenv"

	"github.com/brayanMuniz/AGO2/internal/config"
	"github.com/brayanMuniz/AGO2/internal/danbooru"
	"github.com/brayanMuniz/AGO2/internal/database"
	"github.com/brayanMuniz/AGO2/internal/handler"
)

func main() {
	cfg := config.Default()

	db, err := database.InitDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	_ = godotenv.Load("./.env")

	userName, apiKey := danbooru.GetCredentials(db)
	if userName == "" || apiKey == "" {
		fmt.Println("Warning: Danbooru userName or API key not configured. Please set them via Settings -> Danbooru.")
	}

	mux := http.NewServeMux()
	app := &handler.App{DB: db, Cfg: cfg}
	handler.RegisterRoutes(mux, app)

	port := ":8080"
	fmt.Printf("Server is running on http://localhost%s\n", port)

	err = http.ListenAndServe(port, mux)
	if err != nil {
		log.Fatalf("Server failed to start: %v\n", err)
	}
}
