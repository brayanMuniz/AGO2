package main

import (
	"database/sql"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func SearchImagesByTags(db *sql.DB, searchTokens []string) ([]Image, error) {
	if len(searchTokens) == 0 {
		return nil, fmt.Errorf("no search tokens provided")
	}

	var normalTags []string
	var negatedTags []string
	var ratings []string
	var isFavoriteFilter *bool
	var orderByColor string
	var targetColors []Color

	// Base query joining files to metadata_records
	query := "SELECT f.id FROM files f LEFT JOIN metadata_records m ON f.active_metadata_id = m.id"
	var whereClauses []string
	var args []interface{}

	parseNumericToken := func(column, token, prefix string) error {
		valStr := strings.TrimPrefix(token, prefix)
		op := "="

		if strings.HasPrefix(valStr, ">=") {
			op = ">="
			valStr = valStr[2:]
		} else if strings.HasPrefix(valStr, "<=") {
			op = "<="
			valStr = valStr[2:]
		} else if strings.HasPrefix(valStr, ">") {
			op = ">"
			valStr = valStr[1:]
		} else if strings.HasPrefix(valStr, "<") {
			op = "<"
			valStr = valStr[1:]
		}

		val, err := strconv.Atoi(valStr)
		if err != nil {
			return fmt.Errorf("invalid numeric value in search token: '%s'", token)
		}

		whereClauses = append(whereClauses, fmt.Sprintf("f.%s %s ?", column, op))
		args = append(args, val)
		return nil
	}

	// Separate tokens into categories
	for _, token := range searchTokens {
		lowerToken := strings.ToLower(strings.TrimSpace(token))
		if lowerToken == "" {
			continue
		}

		if strings.HasPrefix(lowerToken, "rating:") {
			ratings = append(ratings, strings.TrimPrefix(lowerToken, "rating:"))
			continue
		}

		if strings.HasPrefix(lowerToken, "width:") {
			if err := parseNumericToken("image_width", lowerToken, "width:"); err != nil {
				return nil, err
			}
			continue
		}

		if strings.HasPrefix(lowerToken, "height:") {
			if err := parseNumericToken("image_height", lowerToken, "height:"); err != nil {
				return nil, err
			}
			continue
		}

		if strings.HasPrefix(lowerToken, "size:") {
			if err := parseNumericToken("file_size", lowerToken, "size:"); err != nil {
				return nil, err
			}
			continue
		}

		if strings.HasPrefix(lowerToken, "favorite:") {
			valStr := strings.TrimPrefix(lowerToken, "favorite:")
			favVal, err := strconv.ParseBool(valStr)
			if err != nil {
				return nil, fmt.Errorf("invalid boolean value in search token: '%s'", token)
			}
			isFavoriteFilter = &favVal
			continue
		}

		if strings.HasPrefix(lowerToken, "brightness:") {
			valStr := strings.TrimPrefix(lowerToken, "brightness:")
			parts := strings.Split(valStr, "-")
			if len(parts) == 2 {
				minB, err1 := strconv.ParseFloat(parts[0], 64)
				maxB, err2 := strconv.ParseFloat(parts[1], 64)
				if err1 == nil && err2 == nil {
					// Use a subquery to calculate the average brightness of the image's palette
					whereClauses = append(whereClauses, `
						(SELECT AVG((0.299 * r) + (0.587 * g) + (0.114 * b)) 
						 FROM image_colors WHERE file_id = f.id) BETWEEN ? AND ?
					`)
					args = append(args, minB, maxB)
				}
			}
			continue
		}

		if strings.HasPrefix(lowerToken, "color:") || strings.HasPrefix(lowerToken, "palette:") {
			prefix := "color:"
			if strings.HasPrefix(lowerToken, "palette:") {
				prefix = "palette:"
			}
			hexStr := strings.TrimPrefix(lowerToken, prefix)
			for _, p := range strings.Split(hexStr, ",") {
				pClean := strings.TrimSpace(p)
				if pClean != "" {
					targetColors = append(targetColors, ParseHexToColor(pClean))
				}
			}
			continue
		}

		if lowerToken == "is:missing" {
			whereClauses = append(whereClauses, "f.active_metadata_id IS NULL AND f.hasDuplicate IS NULL")
			continue
		}

		if lowerToken == "is:duplicate" {
			whereClauses = append(whereClauses, "f.hasDuplicate IS NOT NULL")
			continue
		}

		if lowerToken == "is:organized" {
			whereClauses = append(whereClauses, "f.organized = TRUE")
			continue
		}

		if lowerToken == "is:unorganized" {
			whereClauses = append(whereClauses, "f.organized = FALSE")
			continue
		}

		if strings.HasPrefix(lowerToken, "-") && len(lowerToken) > 1 {
			negatedTags = append(negatedTags, strings.TrimSpace(token[1:]))
			continue
		}

		normalTags = append(normalTags, strings.TrimSpace(token))
	}

	// Handle Rating Filters
	if len(ratings) > 0 {
		placeholders := make([]string, len(ratings))
		for i, r := range ratings {
			placeholders[i] = "?"
			args = append(args, r)
		}
		whereClauses = append(whereClauses, fmt.Sprintf("LOWER(m.rating) IN (%s)", strings.Join(placeholders, ",")))
	}

	// Handle Favorite Filter
	if isFavoriteFilter != nil {
		whereClauses = append(whereClauses, "f.isFavorite = ?")
		args = append(args, *isFavoriteFilter)
	}

	// Handle Tag Filters
	if len(normalTags) > 0 {
		query += `
			JOIN record_tags rt ON m.id = rt.metadata_id
			JOIN tags t ON rt.tag_id = t.id`

		placeholders := make([]string, len(normalTags))
		for i, tag := range normalTags {
			placeholders[i] = "?"
			args = append(args, tag)
		}
		whereClauses = append(whereClauses, fmt.Sprintf("t.name IN (%s)", strings.Join(placeholders, ",")))
	}

	// Handle Negated Tag Filters (exclude images that have these tags)
	for _, negTag := range negatedTags {
		whereClauses = append(whereClauses, `NOT EXISTS (
			SELECT 1 FROM record_tags nrt
			JOIN tags nt ON nrt.tag_id = nt.id
			WHERE nrt.metadata_id = m.id AND nt.name = ?
		)`)
		args = append(args, negTag)
	}

	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	if len(normalTags) > 0 {
		query += " GROUP BY f.id HAVING COUNT(DISTINCT t.id) = ?"
		args = append(args, len(normalTags))
	}

	if orderByColor != "" {
		query += " ORDER BY " + orderByColor + " LIMIT 50"
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer rows.Close()

	var fileIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan file ID: %w", err)
		}
		fileIDs = append(fileIDs, id)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search rows: %w", err)
	}

	if len(targetColors) > 0 {
		allowedIDs := make(map[int64]bool)
		for _, id := range fileIDs {
			allowedIDs[id] = true
		}
		return SearchImagesByPalette(db, targetColors, 35.0, allowedIDs)
	}

	// Fetch full image records
	var images []Image
	for _, id := range fileIDs {
		img, err := GetImageByID(db, id, false)
		if err != nil {
			fmt.Printf("Warning: failed to load image %d: %v\n", id, err)
			continue
		}
		images = append(images, *img)
	}

	return images, nil
}

// SearchImagesByPalette queries color palettes + weights from SQLite and runs in-memory CIE L*a*b* vibe matching.
// Factors in perceptual color distance Delta E and relative weights, dropping images beyond threshold.
func SearchImagesByPalette(db *sql.DB, targetColors []Color, threshold float64, allowedIDs map[int64]bool) ([]Image, error) {
	if len(targetColors) == 0 {
		return nil, nil
	}

	query := `SELECT file_id, r, g, b, hex, weight FROM image_colors`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query image_colors: %w", err)
	}
	defer rows.Close()

	palettes := make(map[int64][]Color)
	for rows.Next() {
		var fileID int64
		var c Color
		if err := rows.Scan(&fileID, &c.R, &c.G, &c.B, &c.Hex, &c.Weight); err != nil {
			continue
		}
		if allowedIDs != nil && len(allowedIDs) > 0 && !allowedIDs[fileID] {
			continue
		}
		palettes[fileID] = append(palettes[fileID], c)
	}

	type matchScore struct {
		fileID   int64
		distance float64
	}
	var matches []matchScore

	for fileID, imgPalette := range palettes {
		if len(imgPalette) == 0 {
			continue
		}

		distT2P := 0.0
		for _, tc := range targetColors {
			minD := math.MaxFloat64
			for _, pc := range imgPalette {
				d := ColorDistanceLAB(tc, pc)
				if d < minD {
					minD = d
				}
			}
			distT2P += minD
		}
		distT2P /= float64(len(targetColors))

		distP2T := 0.0
		totalWeight := 0.0
		for _, pc := range imgPalette {
			minD := math.MaxFloat64
			for _, tc := range targetColors {
				d := ColorDistanceLAB(pc, tc)
				if d < minD {
					minD = d
				}
			}
			w := pc.Weight
			if w <= 0 {
				w = 1.0 / float64(len(imgPalette))
			}
			distP2T += minD * w
			totalWeight += w
		}
		if totalWeight > 0 {
			distP2T /= totalWeight
		}

		vibeDist := (distT2P + distP2T) / 2.0

		if vibeDist <= threshold {
			matches = append(matches, matchScore{fileID: fileID, distance: vibeDist})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].distance < matches[j].distance
	})

	var images []Image
	for _, m := range matches {
		img, err := GetImageByID(db, m.fileID, false)
		if err != nil || img == nil {
			continue
		}
		images = append(images, *img)
	}

	return images, nil
}

// SearchImagesByColor remains for backward compatibility, delegating to perceptual SearchImagesByPalette.
func SearchImagesByColor(db *sql.DB, targetR, targetG, targetB, threshold int) ([]Image, error) {
	c := Color{
		R:      targetR,
		G:      targetG,
		B:      targetB,
		Hex:    fmt.Sprintf("#%02x%02x%02x", targetR, targetG, targetB),
		Weight: 1.0,
	}
	// Default strict cutoff around 35.0 perceptual Delta E if threshold <= 0
	cut := float64(threshold)
	if cut <= 0 {
		cut = 35.0
	}
	return SearchImagesByPalette(db, []Color{c}, cut, nil)
}

// Brightness ranges from 0 (pitch black) to 255 (pure white).
func SearchImagesByBrightness(db *sql.DB, minBrightness, maxBrightness float64) ([]Image, error) {
	query := `
		SELECT file_id, 
		       AVG((0.299 * r) + (0.587 * g) + (0.114 * b)) as avg_brightness
		FROM image_colors
		GROUP BY file_id
		HAVING avg_brightness >= ? AND avg_brightness <= ?
		ORDER BY avg_brightness ASC
		LIMIT 50
	`

	rows, err := db.Query(query, minBrightness, maxBrightness)
	if err != nil {
		return nil, fmt.Errorf("failed to query brightness: %w", err)
	}
	defer rows.Close()

	var fileIDs []int64
	for rows.Next() {
		var id int64
		var avgBrightness float64
		if err := rows.Scan(&id, &avgBrightness); err != nil {
			return nil, err
		}
		fileIDs = append(fileIDs, id)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating brightness rows: %w", err)
	}

	// Fetch full image records using your existing function
	var images []Image
	for _, id := range fileIDs {
		img, err := GetImageByID(db, id, false)
		if err != nil {
			fmt.Printf("Warning: failed to load image %d: %v\n", id, err)
			continue
		}
		images = append(images, *img)
	}

	return images, nil
}

// ExtractCombinedPaletteFromImages takes a list of file IDs, fetches their extracted colors
// (or extracts on-the-fly if missing), clusters perceptually similar colors using ColorDistanceLAB,
// and guarantees returning at least 5 representative hex colors ranked by dominant weight.
func ExtractCombinedPaletteFromImages(db *sql.DB, fileIDs []int64) ([]string, error) {
	if len(fileIDs) == 0 {
		return []string{}, nil
	}

	type colorEntry struct {
		c      Color
		weight float64
	}
	var allColors []colorEntry

	for _, id := range fileIDs {
		rows, err := db.Query("SELECT r, g, b, hex, weight FROM image_colors WHERE file_id = ?", id)
		if err == nil {
			for rows.Next() {
				var c Color
				var w float64
				if err := rows.Scan(&c.R, &c.G, &c.B, &c.Hex, &w); err == nil {
					allColors = append(allColors, colorEntry{c: c, weight: w})
				}
			}
			rows.Close()
		}

		// If DB didn't provide enough colors, extract more directly from the file
		if len(allColors) < 10 {
			var filename string
			if err := db.QueryRow("SELECT filename FROM files WHERE id = ?", id).Scan(&filename); err == nil {
				filePath := filepath.Join("Gallery", filename)
				if pal, err := ExtractColorPalette(filePath, 10); err == nil {
					for _, c := range pal {
						allColors = append(allColors, colorEntry{c: c, weight: c.Weight})
					}
				}
			}
		}
	}

	if len(allColors) == 0 {
		return []string{"#1e1e2e", "#cba6f7", "#f38ba8", "#a6e3a1", "#89b4fa"}, nil
	}

	// Sort candidate colors by weight descending
	sort.Slice(allColors, func(i, j int) bool {
		return allColors[i].weight > allColors[j].weight
	})

	type cluster struct {
		c      Color
		weight float64
	}
	var clusters []cluster

	// First pass: cluster with moderate perceptual distance
	for _, entry := range allColors {
		merged := false
		for idx, cl := range clusters {
			if ColorDistanceLAB(entry.c, cl.c) < 12.0 {
				clusters[idx].weight += entry.weight
				merged = true
				break
			}
		}
		if !merged {
			clusters = append(clusters, cluster{c: entry.c, weight: entry.weight})
		}
	}

	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].weight > clusters[j].weight
	})

	var result []string
	seenHex := make(map[string]bool)

	for _, cl := range clusters {
		hex := strings.ToLower(cl.c.Hex)
		if !seenHex[hex] {
			seenHex[hex] = true
			result = append(result, cl.c.Hex)
		}
		if len(result) >= 5 {
			break
		}
	}

	// Second pass: if fewer than 5, include remaining unique hexes from allColors
	for _, entry := range allColors {
		if len(result) >= 5 {
			break
		}
		hex := strings.ToLower(entry.c.Hex)
		if !seenHex[hex] {
			seenHex[hex] = true
			result = append(result, entry.c.Hex)
		}
	}

	// Third pass: if STILL fewer than 5 (e.g. single monochromatic image), generate harmonious tints/shades
	if len(result) < 5 && len(allColors) > 0 {
		base := allColors[0].c
		variations := []Color{
			{R: int(math.Min(255, float64(base.R)+float64(255-base.R)*0.35)), G: int(math.Min(255, float64(base.G)+float64(255-base.G)*0.35)), B: int(math.Min(255, float64(base.B)+float64(255-base.B)*0.35))},
			{R: int(float64(base.R) * 0.65), G: int(float64(base.G) * 0.65), B: int(float64(base.B) * 0.65)},
			{R: int(math.Min(255, float64(base.R)+float64(255-base.R)*0.6)), G: int(math.Min(255, float64(base.G)+float64(255-base.G)*0.6)), B: int(math.Min(255, float64(base.B)+float64(255-base.B)*0.6))},
			{R: int(float64(base.R) * 0.4), G: int(float64(base.G) * 0.4), B: int(float64(base.B) * 0.4)},
			{R: base.B, G: base.R, B: base.G}, // subtle rotated accent
		}
		for _, v := range variations {
			if len(result) >= 5 {
				break
			}
			hex := fmt.Sprintf("#%02x%02x%02x", v.R, v.G, v.B)
			if !seenHex[strings.ToLower(hex)] {
				seenHex[strings.ToLower(hex)] = true
				result = append(result, hex)
			}
		}
	}

	return result, nil
}
