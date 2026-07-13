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
	"strings"
	"time"
)

// MetadataProvider abstracts image reverse-search and tag lookup providers.
type MetadataProvider interface {
	SearchByFile(filePath string) (IQDBResponse, error)
}

type DanbooruIQDBProvider struct {
	APIKey   string
	UserName string
}

func (d *DanbooruIQDBProvider) SearchByFile(filePath string) (IQDBResponse, error) {
	return iqdb_upload_request(d.APIKey, d.UserName, filePath)
}

type IQDBResponse []IQDBMatch

type IQDBMatch struct {
	PostID int     `json:"post_id"`
	Score  float64 `json:"score"`
	Post   Post    `json:"post"`
}

// MarshalJSON customizes the JSON output to hide specific fields from the frontend response
func (p Post) MarshalJSON() ([]byte, error) {
	type Alias Post

	return json.Marshal(&struct {
		Alias
		RawTagStringArtist    string `json:"tag_string_artist,omitempty"`
		RawTagStringCharacter string `json:"tag_string_character,omitempty"`
		RawTagStringCopyright string `json:"tag_string_copyright,omitempty"`
		RawTagStringGeneral   string `json:"tag_string_general,omitempty"`
		RawTagStringMeta      string `json:"tag_string_meta,omitempty"`
	}{
		Alias:                 (Alias)(p),
		RawTagStringArtist:    "",
		RawTagStringCharacter: "",
		RawTagStringCopyright: "",
		RawTagStringGeneral:   "",
		RawTagStringMeta:      "",
	})
}

// intercepts the default unmarshaling to run SplitRawStrings() automatically.
func (m *IQDBMatch) UnmarshalJSON(data []byte) error {
	// Create an alias type to prevent infinite loop recursion
	type Alias IQDBMatch
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	m.Post.SplitRawStrings()
	return nil
}

type Post struct {
	ID int `json:"id"`

	FileURL        string `json:"file_url"`
	LargeFileURL   string `json:"large_file_url"`
	PreviewFileURL string `json:"preview_file_url"`

	Rating      string `json:"rating"`
	Source      string `json:"source"`
	ImageHeight int    `json:"image_height"`
	ImageWidth  int    `json:"image_width"`
	FileSize    int    `json:"file_size"`

	OriginalPostID string `json:"original_post_id,omitempty"`
	OriginalSource string `json:"original_source,omitempty"`

	TagsArtist     []string `json:"tags_artist"`
	TagsCharacters []string `json:"tags_character"`
	TagsCopyright  []string `json:"tags_copyright"`
	TagsGeneral    []string `json:"tags_general"`
	TagsMeta       []string `json:"tags_meta"`

	TagCount          int `json:"tag_count"`
	TagCountArtist    int `json:"tag_count_artist"`
	TagCountCharacter int `json:"tag_count_character"`
	TagCountCopyright int `json:"tag_count_copyright"`
	TagCountGeneral   int `json:"tag_count_general"`
	TagCountMeta      int `json:"tag_count_meta"`

	RawTagStringArtist    string `json:"tag_string_artist"`
	RawTagStringCharacter string `json:"tag_string_character"`
	RawTagStringCopyright string `json:"tag_string_copyright"`
	RawTagStringGeneral   string `json:"tag_string_general"`
	RawTagStringMeta      string `json:"tag_string_meta"`
}

func splitTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	return strings.Fields(raw)
}

func (p *Post) SplitRawStrings() {
	p.TagsArtist = splitTags(p.RawTagStringArtist)
	p.TagsCharacters = splitTags(p.RawTagStringCharacter)
	p.TagsCopyright = splitTags(p.RawTagStringCopyright)
	p.TagsGeneral = splitTags(p.RawTagStringGeneral)
	p.TagsMeta = splitTags(p.RawTagStringMeta)

	p.TagCountArtist = len(p.TagsArtist)
	p.TagCountCharacter = len(p.TagsCharacters)
	p.TagCountCopyright = len(p.TagsCopyright)
	p.TagCountGeneral = len(p.TagsGeneral)
	p.TagCountMeta = len(p.TagsMeta)
	p.TagCount = p.TagCountArtist + p.TagCountCharacter + p.TagCountCopyright + p.TagCountGeneral + p.TagCountMeta
}

func iqdb_upload_request(apiKey, userName, fileLocation string) (IQDBResponse, error) {
	base_url := "https://danbooru.donmai.us/iqdb_queries.json"
	u, err := url.Parse(base_url)
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
		fmt.Printf("", err)
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
		fmt.Printf("Response code is %s", resp.StatusCode)
		return nil, err
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var matches IQDBResponse
	err = json.Unmarshal(respBytes, &matches)
	if err != nil {
		fmt.Printf("Failed to parse response as IQDBResponse: %v\n", err)
		return nil, err
	}

	return matches, nil
}
