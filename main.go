package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type IQDBResponse []IQDBMatch

type IQDBMatch struct {
	PostID int     `json:"post_id"`
	Score  float64 `json:"score"`
	Post   Post    `json:"post"`
}

type Post struct {
	ID           int    `json:"id"`
	FileURL      string `json:"file_url"`
	LargeFileURL string `json:"large_file_url"`
	Rating       string `json:"rating"` // 's' (Safe), 'q' (Questionable), 'e' (Explicit)
	Source       string `json:"source"`
	ImageHeight  int    `json:"image_height"`
	ImageWidth   int    `json:"image_width"`
	FileSize     int    `json:"file_size"`

	TagStringArtist     string `json:"tag_string_artist"`
	TagStringCharacter  string `json:"tag_string_character"`
	TagStringCopyright  string `json:"tag_string_copyright"`
	RawTagString        string `json:"tag_string"`         // Needs to be split into an array
	RawTagStringGeneral string `json:"tag_string_general"` // Needs to be split into an array

	TagCount          int    `json:"tag_count"`
	TagCountArtist    int    `json:"tag_count_artist"`
	TagCountCharacter int    `json:"tag_count_character"`
	TagCountCopyright int    `json:"tag_count_copyright"`
	TagCountGeneral   int    `json:"tag_count_general"`
	TagCountMeta      int    `json:"tag_count_meta"`
	TagStringMeta     string `json:"tag_string_meta"`
}

func main() {
	err := godotenv.Load("./.env")
	if err != nil {
		fmt.Println("Could not read the .env file")
		return
	}

	userName := os.Getenv("USERNAME")
	if userName == "" {
		fmt.Println("Username is empty")
		return
	}

	apiKey := os.Getenv("DANBOORU_KEY")
	if apiKey == "" {
		fmt.Println("api key is empty")
		return
	}

	// iqdb_upload_request(apiKey, userName)
	readJsonFile()

}

func readJsonFile() {
	filePath := "response.json"
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	var matches IQDBResponse
	err = json.Unmarshal(fileBytes, &matches)
	if err != nil {
		fmt.Printf("Error parsing JSON: %v\n", err)
		return
	}

	fmt.Printf("Found %d potential matches!\n", len(matches))
	fmt.Println("========================================")
	for i, match := range matches {
		if match.Score == 100 {
			fmt.Printf("File Size: %v\n", match.Post.FileSize)
			fmt.Printf("Image Height: %v\n", match.Post.ImageHeight)
			fmt.Printf("Image Width: %v\n", match.Post.ImageWidth)

			fmt.Printf("Raw Tags: %s\n", match.Post.RawTagString)
			fmt.Printf("General: %s\n", match.Post.RawTagStringGeneral)

			fmt.Printf("Match #%d (Confidence Score: %.2f)\n", i+1, match.Score)
			fmt.Printf("Post ID:    %d\n", match.Post.ID)
			fmt.Printf("Rating:     %s\n", match.Post.Rating)

			if match.Post.TagStringArtist != "" {
				fmt.Printf("Artist:     %s\n", match.Post.TagStringArtist)
			}
			if match.Post.TagStringCharacter != "" {
				fmt.Printf("Character:  %s\n", match.Post.TagStringCharacter)
			}
			if match.Post.TagStringCopyright != "" {
				fmt.Printf("Franchise:  %s\n", match.Post.TagStringCopyright)
			}

			fmt.Printf("  Source:     %s\n", match.Post.Source)
			fmt.Printf("  Image URL:  %s\n", match.Post.FileURL)
			fmt.Println("----------------------------------------")

		}
	}
}

func iqdb_upload_request(apiKey, userName string) {
	base_url := "https://danbooru.donmai.us/iqdb_queries.json"
	u, err := url.Parse(base_url)
	if err != nil {
		fmt.Println("Error parsing URL:", err)
		return
	}
	q := u.Query()
	q.Set("login", userName)
	q.Set("api_key", apiKey)
	u.RawQuery = q.Encode()

	testFile := "./assets/test.jpg"

	f, err := os.Open(testFile)
	if err != nil {
		fmt.Printf("", err)
		return
	}
	defer f.Close()

	bodyBuffer := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuffer)

	fileWriter, err := bodyWriter.CreateFormFile("search[file]", testFile)
	if err != nil {
		fmt.Println("Error creating form file field:", err)
		return
	}

	if _, err = io.Copy(fileWriter, f); err != nil {
		fmt.Println("Error copying file data:", err)
		return
	}
	bodyWriter.Close()

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", u.String(), bodyBuffer)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	req.Header.Set("Content-Type", bodyWriter.FormDataContentType())
	req.Header.Set("User-Agent", "MyGoApp/1.0")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Network error:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		fmt.Println("Response code is not OK for Authentication")
		fmt.Printf("Response code is %s", resp.StatusCode)
		return
	}

	respBytes, _ := io.ReadAll(resp.Body)
	var rawJSON interface{}
	err = json.Unmarshal(respBytes, &rawJSON)
	if err != nil {
		fmt.Printf("Failed to parse raw response as JSON: %v\n", err)
		return
	}

	prettyJSON, err := json.MarshalIndent(rawJSON, "", "    ")
	if err != nil {
		fmt.Printf("Failed to format JSON: %v\n", err)
		return
	}

	outputFile := "response.json"
	err = os.WriteFile(outputFile, prettyJSON, 0644)
	if err != nil {
		fmt.Printf("Error writing response to file: %v\n", err)
		return
	}
}
