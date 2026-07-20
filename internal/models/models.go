package models

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
)

// --- Core Domain Types ---

// Color represents an extracted color with RGB values, hex string, and relative weight.
type Color struct {
	R      int     `json:"r"`
	G      int     `json:"g"`
	B      int     `json:"b"`
	Hex    string  `json:"hex"`
	Weight float64 `json:"weight"`
}

// Image represents a file record from the database.
type Image struct {
	ID            int64  `json:"id"`
	FileName      string `json:"file_name"`
	IsFavorite    bool   `json:"is_favorite"`
	Organized     bool   `json:"organized"`
	HasDuplicate  *int64 `json:"has_duplicate,omitempty"`
	Hash          string `json:"hash"`
	MainData      *Post  `json:"main_data"`
	ThumbnailPath string `json:"thumbnail_path"`
	ImageWidth    int    `json:"image_width"`
	ImageHeight   int    `json:"image_height"`
	FileSize      int64  `json:"file_size"`
}

// --- Danbooru / IQDB Types ---

// Post represents metadata from a Danbooru post or custom metadata entry.
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

// MarshalJSON customizes the JSON output to hide raw tag string fields from the frontend response.
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

func splitTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	return strings.Fields(raw)
}

// SplitRawStrings populates the typed tag slices from the raw space-delimited strings.
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

// IQDBResponse is the list of matches returned by the Danbooru IQDB API.
type IQDBResponse []IQDBMatch

// IQDBMatch represents a single match result from IQDB.
type IQDBMatch struct {
	PostID int     `json:"post_id"`
	Score  float64 `json:"score"`
	Post   Post    `json:"post"`
}

// UnmarshalJSON intercepts the default unmarshaling to run SplitRawStrings() automatically.
func (m *IQDBMatch) UnmarshalJSON(data []byte) error {
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

// MetadataProvider abstracts image reverse-search and tag lookup providers.
type MetadataProvider interface {
	SearchByFile(filePath string) (IQDBResponse, error)
}

// --- Processing Types ---

// ProcessedImage holds the result of processing a single image during gallery sync.
type ProcessedImage struct {
	AutoMatch bool
	Skipped   bool
}

// UpdateImageParams contains the optional fields that can be updated on an image.
type UpdateImageParams struct {
	IsFavorite       *bool
	ActiveMetadataID *int64 // Use 0 or a negative number to clear the metadata
	MainData         *Post
	ReplaceImage     *bool
}

// --- Gallery Worker Types ---

// ProcessGallerySum tracks counts during a gallery sync job.
type ProcessGallerySum struct {
	Processed int `json:"processed"`
	AutoMatch int `json:"auto_match"`
	Skipped   int `json:"skipped"`
}

// JobState tracks the state of a background gallery processing job.
type JobState struct {
	sync.RWMutex
	ID         string            `json:"job_id"`
	Status     string            `json:"status"` // "processing", "completed", "failed"
	Stats      ProcessGallerySum `json:"stats"`
	TotalFiles int               `json:"total_files"`
	Error      string            `json:"error,omitempty"`
}

// --- Tag Autocomplete ---

// TagSuggestion represents a tag entry for autocomplete results.
type TagSuggestion struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Count    int    `json:"count,omitempty"`
}

// --- Saved Filters & Palettes ---

// SavedFilter represents a user-saved search filter preset.
type SavedFilter struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Query     string `json:"query"`
	SortBy    string `json:"sort_by"`
	SortOrder string `json:"sort_order"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// SavedPalette represents a user-saved color palette.
type SavedPalette struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Colors    []string `json:"colors"`
	CreatedAt string   `json:"created_at"`
}

// --- Stats Types ---

// LibraryStats holds aggregate counts for the entire library.
type LibraryStats struct {
	TotalImages      int   `json:"total_images"`
	TotalDuplicates  int   `json:"total_duplicates"`
	TotalFavorites   int   `json:"total_favorites"`
	TotalDiskSpace   int64 `json:"total_disk_space"`
	UnorganizedQueue int   `json:"unorganized_queue"`
}

// TagLeaderboardEntry represents a tag with its frequency count.
type TagLeaderboardEntry struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Count    int    `json:"count"`
}

// RatingDistribution holds a rating value and its count.
type RatingDistribution struct {
	Rating string `json:"rating"`
	Count  int    `json:"count"`
}

// PredictiveTagEntry holds a tag's rating distribution percentages.
type PredictiveTagEntry struct {
	Name            string  `json:"name"`
	TotalCount      int     `json:"total_count"`
	GeneralPct      float64 `json:"general_pct"`
	SensitivePct    float64 `json:"sensitive_pct"`
	QuestionablePct float64 `json:"questionable_pct"`
	ExplicitPct     float64 `json:"explicit_pct"`
}

// ArtistProfile holds statistics about a single artist.
type ArtistProfile struct {
	Name            string                `json:"name"`
	TotalCount      int                   `json:"total_count"`
	FavoriteCount   int                   `json:"favorite_count"`
	RatingBreakdown map[string]int        `json:"rating_breakdown"`
	TopTags         []TagLeaderboardEntry `json:"top_tags"`
}

// StatsPayload is the combined response for the /api/stats endpoint.
type StatsPayload struct {
	Library            LibraryStats                     `json:"library"`
	TagLeaderboards    map[string][]TagLeaderboardEntry `json:"tag_leaderboards"`
	TagLeaderboardsFav map[string][]TagLeaderboardEntry `json:"tag_leaderboards_favorites"`
	RatingDist         []RatingDistribution             `json:"rating_distribution"`
	PredictiveByRating map[string][]PredictiveTagEntry  `json:"predictive_by_rating"`
	ArtistProfiles     []ArtistProfile                  `json:"artist_profiles"`
}

// --- Color Science ---

// RGBToLAB converts standard 8-bit sRGB (0-255) to CIE L*a*b* (D65 illuminant).
func RGBToLAB(r, g, b int) (l, a, bVal float64) {
	lin := func(c int) float64 {
		v := float64(c) / 255.0
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}

	rl := lin(r)
	gl := lin(g)
	bl := lin(b)

	x := (0.4124564*rl + 0.3575761*gl + 0.1804375*bl) / 0.95047
	y := (0.2126729*rl + 0.7151522*gl + 0.0721750*bl) / 1.00000
	z := (0.0193339*rl + 0.1191920*gl + 0.9503041*bl) / 1.08883

	f := func(t float64) float64 {
		if t > 0.008856 {
			return math.Cbrt(t)
		}
		return (7.787 * t) + (16.0 / 116.0)
	}

	fx := f(x)
	fy := f(y)
	fz := f(z)

	l = (116.0 * fy) - 16.0
	a = 500.0 * (fx - fy)
	bVal = 200.0 * (fy - fz)
	return l, a, bVal
}

// ColorDistanceLAB computes perceptual Euclidean distance Delta E in CIE L*a*b* space.
func ColorDistanceLAB(c1, c2 Color) float64 {
	l1, a1, b1 := RGBToLAB(c1.R, c1.G, c1.B)
	l2, a2, b2 := RGBToLAB(c2.R, c2.G, c2.B)
	dl := l1 - l2
	da := a1 - a2
	db := b1 - b2
	return math.Sqrt(dl*dl + da*da + db*db)
}

// ParseHexToColor converts "#RRGGBB" or "RRGGBB" into a Color struct.
func ParseHexToColor(hexStr string) Color {
	clean := strings.TrimPrefix(hexStr, "#")
	var r, g, b int
	if len(clean) == 6 {
		fmt.Sscanf(clean, "%02x%02x%02x", &r, &g, &b)
	}
	return Color{
		R:      r,
		G:      g,
		B:      b,
		Hex:    "#" + clean,
		Weight: 1.0,
	}
}
