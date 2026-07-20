package danbooru

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

	"github.com/brayanMuniz/AGO2/internal/models"
)

// DanbooruIQDBProvider implements MetadataProvider using the Danbooru IQDB API.
type DanbooruIQDBProvider struct {
	APIKey   string
	UserName string
}

// SearchByFile uploads an image to Danbooru IQDB and returns match results.
func (d *DanbooruIQDBProvider) SearchByFile(filePath string) (models.IQDBResponse, error) {
	return IQDBUploadRequest(d.APIKey, d.UserName, filePath)
}

// IQDBUploadRequest uploads a local image file to the Danbooru IQDB endpoint
// and returns the list of matches.
func IQDBUploadRequest(apiKey, userName, fileLocation string) (models.IQDBResponse, error) {
	baseURL := "https://danbooru.donmai.us/iqdb_queries.json"
	u, err := url.Parse(baseURL)
	if err != nil {
		fmt.Println("Error parsing URL:", err)
		return nil, err
	}
	q := u.Query()
	q.Set("login", userName)
	q.Set("api_key", apiKey)
	u.RawQuery = q.Encode()

	f, err := os.Open(fileLocation)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return nil, err
	}
	defer f.Close()

	bodyBuffer := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuffer)

	fileWriter, err := bodyWriter.CreateFormFile("search[file]", fileLocation)
	if err != nil {
		fmt.Println("Error creating form file field:", err)
		return nil, err
	}

	if _, err = io.Copy(fileWriter, f); err != nil {
		fmt.Println("Error copying file data:", err)
		return nil, err
	}
	bodyWriter.Close()

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", u.String(), bodyBuffer)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return nil, err
	}
	req.Header.Set("Content-Type", bodyWriter.FormDataContentType())
	req.Header.Set("User-Agent", "MyGoApp/1.0")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Network error:", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		fmt.Println("Response code is not OK for Authentication")
		fmt.Printf("Response code is %d\n", resp.StatusCode)
		return nil, fmt.Errorf("IQDB API returned status %d", resp.StatusCode)
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var matches models.IQDBResponse
	err = json.Unmarshal(respBytes, &matches)
	if err != nil {
		fmt.Printf("Failed to parse response as IQDBResponse: %v\n", err)
		return nil, err
	}

	return matches, nil
}
