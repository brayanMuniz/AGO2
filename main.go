package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"os"
)

func main() {
	database, err := InitDB("./galleryDB.db")
	if err != nil {
		fmt.Printf("", err)
	}
	defer database.Close()

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

	// TEST: PASS
	testFile := "test3.jpg"
	testPath := "./assets/" + testFile

	fmt.Println("Processing new upload...")
	err = ProcessNewUpload(database, apiKey, userName, testFile, testPath)
	if err != nil {
		fmt.Printf("Upload Error: %v\n", err)
	}

	fmt.Println("\nFetching from Database...")
	profile, err := GetApprovedMetadata(database, testFile)
	if err != nil {
		fmt.Printf("Query Error: %v\n", err)
	} else {
		fmt.Printf("Success! Loaded Profile from DB.\nSource: %s\nRating: %s\nDimensions: %dx%d\n",
			profile.Source, profile.Rating, profile.ImageWidth, profile.ImageHeight)
	}
}
