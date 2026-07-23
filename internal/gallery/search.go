package gallery

import (
	"database/sql"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/brayanMuniz/AGO2/internal/models"
	"github.com/brayanMuniz/AGO2/internal/storage"
)

// SearchImages parses search tokens and returns matching images along with total matching count.
func SearchImages(db *sql.DB, searchTokens []string, limit int, offset int, sortBy string, sortOrder string) ([]models.Image, int, error) {
	if len(searchTokens) == 0 {
		return nil, 0, fmt.Errorf("no search tokens provided")
	}

	var normalTags []string
	var negatedTags []string
	var ratings []string
	var isFavoriteFilter *bool
	var orderByColor string
	var targetColors []models.Color
	var paletteThreshold float64 = 18.0

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
				return nil, 0, err
			}
			continue
		}

		if strings.HasPrefix(lowerToken, "height:") {
			if err := parseNumericToken("image_height", lowerToken, "height:"); err != nil {
				return nil, 0, err
			}
			continue
		}

		if strings.HasPrefix(lowerToken, "size:") {
			if err := parseNumericToken("file_size", lowerToken, "size:"); err != nil {
				return nil, 0, err
			}
			continue
		}

		if strings.HasPrefix(lowerToken, "favorite:") {
			valStr := strings.TrimPrefix(lowerToken, "favorite:")
			favVal, err := strconv.ParseBool(valStr)
			if err != nil {
				return nil, 0, fmt.Errorf("invalid boolean value in search token: '%s'", token)
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
					targetColors = append(targetColors, models.ParseHexToColor(pClean))
				}
			}
			continue
		}

		if strings.HasPrefix(lowerToken, "palette_threshold:") || strings.HasPrefix(lowerToken, "color_threshold:") {
			prefix := "palette_threshold:"
			if strings.HasPrefix(lowerToken, "color_threshold:") {
				prefix = "color_threshold:"
			}
			valStr := strings.TrimPrefix(lowerToken, prefix)
			if val, err := strconv.ParseFloat(valStr, 64); err == nil && val > 0 {
				paletteThreshold = val
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

	// Calculate total matching count before applying LIMIT/OFFSET/ORDER BY
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS count_sub", query)
	var totalCount int
	if err := db.QueryRow(countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	if len(targetColors) > 0 {
		// Run without pagination first to get allowed IDs, then delegate to palette matching which handles limit/offset after distance sorting
		rows, err := db.Query(query, args...)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to search for palette allowed IDs: %w", err)
		}
		defer rows.Close()

		allowedIDs := make(map[int64]bool)
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return nil, 0, fmt.Errorf("failed to scan file ID: %w", err)
			}
			allowedIDs[id] = true
		}
		if err = rows.Err(); err != nil {
			return nil, 0, fmt.Errorf("error iterating search rows: %w", err)
		}
		return SearchImagesByPalette(db, targetColors, paletteThreshold, allowedIDs, limit, offset)
	}

	// Apply SQL ORDER BY
	orderClause := ""
	if orderByColor != "" {
		orderClause = " ORDER BY " + orderByColor
	} else if sortBy != "" && sortBy != "none" {
		dir := "DESC"
		if strings.ToLower(sortOrder) == "asc" {
			dir = "ASC"
		}
		switch sortBy {
		case "created_at":
			orderClause = fmt.Sprintf(" ORDER BY f.created_at %s, f.id %s", dir, dir)
		case "id":
			orderClause = fmt.Sprintf(" ORDER BY f.id %s", dir)
		case "file_size":
			orderClause = fmt.Sprintf(" ORDER BY f.file_size %s, f.id %s", dir, dir)
		case "dimensions":
			orderClause = fmt.Sprintf(" ORDER BY (f.image_width * f.image_height) %s, f.id %s", dir, dir)
		case "rating":
			orderClause = fmt.Sprintf(" ORDER BY CASE LOWER(m.rating) WHEN 'g' THEN 1 WHEN 's' THEN 2 WHEN 'q' THEN 3 WHEN 'e' THEN 4 ELSE 0 END %s, f.id %s", dir, dir)
		case "random":
			orderClause = " ORDER BY RANDOM()"
		default:
			orderClause = " ORDER BY f.id DESC"
		}
	} else {
		orderClause = " ORDER BY f.id DESC"
	}
	query += orderClause

	if limit > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search: %w", err)
	}
	defer rows.Close()

	var fileIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, 0, fmt.Errorf("failed to scan file ID: %w", err)
		}
		fileIDs = append(fileIDs, id)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating search rows: %w", err)
	}

	// Fetch full image records in batch
	images, err := storage.GetImagesByIDs(db, fileIDs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to batch load images: %w", err)
	}

	return images, totalCount, nil
}

// SearchImagesByPalette queries color palettes + weights from SQLite and runs in-memory CIE L*a*b* vibe matching.
// Factors in perceptual color distance Delta E and relative weights, dropping images beyond threshold.
func SearchImagesByPalette(db *sql.DB, targetColors []models.Color, threshold float64, allowedIDs map[int64]bool, limit int, offset int) ([]models.Image, int, error) {
	if len(targetColors) == 0 {
		return nil, 0, nil
	}
	if allowedIDs != nil && len(allowedIDs) == 0 {
		return []models.Image{}, 0, nil
	}

	query := `SELECT file_id, r, g, b, hex, weight FROM image_colors`
	var args []interface{}
	if allowedIDs != nil && len(allowedIDs) > 0 && len(allowedIDs) <= 900 {
		placeholders := make([]string, 0, len(allowedIDs))
		for id := range allowedIDs {
			placeholders = append(placeholders, "?")
			args = append(args, id)
		}
		query += fmt.Sprintf(" WHERE file_id IN (%s)", strings.Join(placeholders, ","))
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query image_colors: %w", err)
	}
	defer rows.Close()

	palettes := make(map[int64][]models.Color)
	for rows.Next() {
		var fileID int64
		var c models.Color
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
		maxTargetDist := 0.0
		for _, tc := range targetColors {
			minD := math.MaxFloat64
			for _, pc := range imgPalette {
				d := models.ColorDistanceLAB(tc, pc)
				if d < minD {
					minD = d
				}
			}
			distT2P += minD
			if minD > maxTargetDist {
				maxTargetDist = minD
			}
		}
		distT2P /= float64(len(targetColors))

		// Strict rejection: If any requested target color is missing entirely from the image,
		// we reject immediately. A color is considered missing if the closest patch in the image
		// is further than threshold + tolerance (or at least 24.0 Delta E).
		maxAllowedMissingDist := math.Max(threshold*1.35, 24.0)
		if maxTargetDist > maxAllowedMissingDist {
			continue
		}

		distP2T := 0.0
		totalWeight := 0.0
		for _, pc := range imgPalette {
			minD := math.MaxFloat64
			for _, tc := range targetColors {
				d := models.ColorDistanceLAB(pc, tc)
				if d < minD {
					minD = d
				}
			}
			w := pc.Weight
			if w <= 0 {
				w = 1.0 / float64(len(imgPalette))
			}
			// Heavily penalize dominant foreign colors inside the image (e.g. large blue patches when searching red/black)
			if minD > threshold*1.25 && w >= 0.15 {
				minD *= 1.4
			}
			distP2T += minD * w
			totalWeight += w
		}
		if totalWeight > 0 {
			distP2T /= totalWeight
		}

		// Combined strict vibe score: 40% overall target fit, 30% worst target fit (ensures no color ignored),
		// 30% image palette purity (penalizes unrelated colors like blue).
		vibeDist := 0.40*distT2P + 0.30*maxTargetDist + 0.30*distP2T

		if vibeDist <= threshold {
			matches = append(matches, matchScore{fileID: fileID, distance: vibeDist})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].distance < matches[j].distance
	})

	totalCount := len(matches)
	if offset >= totalCount {
		return []models.Image{}, totalCount, nil
	}

	end := offset + limit
	if limit <= 0 || end > totalCount {
		end = totalCount
	}
	slicedMatches := matches[offset:end]

	var resultIDs []int64
	for _, m := range slicedMatches {
		resultIDs = append(resultIDs, m.fileID)
	}

	images, err := storage.GetImagesByIDs(db, resultIDs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to batch load palette matched images: %w", err)
	}

	return images, totalCount, nil
}

// SearchImagesByColor searches for images matching a single target color.
func SearchImagesByColor(db *sql.DB, targetR, targetG, targetB, threshold int) ([]models.Image, error) {
	c := models.Color{
		R:      targetR,
		G:      targetG,
		B:      targetB,
		Hex:    fmt.Sprintf("#%02x%02x%02x", targetR, targetG, targetB),
		Weight: 1.0,
	}
	cut := float64(threshold)
	if cut <= 0 {
		cut = 18.0
	}
	images, _, err := SearchImagesByPalette(db, []models.Color{c}, cut, nil, 0, 0)
	return images, err
}

// SearchImagesByBrightness searches for images within a brightness range.
// Brightness ranges from 0 (pitch black) to 255 (pure white).
func SearchImagesByBrightness(db *sql.DB, minBrightness, maxBrightness float64) ([]models.Image, error) {
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

	// Fetch full image records using batch loader
	images, err := storage.GetImagesByIDs(db, fileIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to batch load brightness matched images: %w", err)
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
		c      models.Color
		weight float64
	}
	var allColors []colorEntry
	colorsByFile := make(map[int64]int)
	chunkSize := 500

	for i := 0; i < len(fileIDs); i += chunkSize {
		end := i + chunkSize
		if end > len(fileIDs) {
			end = len(fileIDs)
		}
		chunk := fileIDs[i:end]

		placeholders := make([]string, len(chunk))
		args := make([]interface{}, len(chunk))
		for j, id := range chunk {
			placeholders[j] = "?"
			args[j] = id
		}

		rows, err := db.Query(fmt.Sprintf("SELECT file_id, r, g, b, hex, weight FROM image_colors WHERE file_id IN (%s)", strings.Join(placeholders, ",")), args...)
		if err == nil {
			for rows.Next() {
				var fID int64
				var c models.Color
				var w float64
				if err := rows.Scan(&fID, &c.R, &c.G, &c.B, &c.Hex, &w); err == nil {
					allColors = append(allColors, colorEntry{c: c, weight: w})
					colorsByFile[fID]++
				}
			}
			rows.Close()
		}
	}

	// If DB didn't provide enough colors, extract more directly from files for missing IDs
	if len(allColors) < 10 {
		var missingIDs []interface{}
		var missingPlaceholders []string
		for _, id := range fileIDs {
			if colorsByFile[id] == 0 {
				missingIDs = append(missingIDs, id)
				missingPlaceholders = append(missingPlaceholders, "?")
				if len(missingIDs) >= 20 {
					break
				}
			}
		}
		if len(missingIDs) > 0 {
			rows, err := db.Query(fmt.Sprintf("SELECT filename FROM files WHERE id IN (%s)", strings.Join(missingPlaceholders, ",")), missingIDs...)
			if err == nil {
				for rows.Next() {
					var filename string
					if err := rows.Scan(&filename); err == nil {
						filePath := filepath.Join("Gallery", filename)
						if pal, err := ExtractColorPalette(filePath, 10); err == nil {
							for _, c := range pal {
								allColors = append(allColors, colorEntry{c: c, weight: c.Weight})
							}
						}
					}
				}
				rows.Close()
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
		c      models.Color
		weight float64
	}
	var clusters []cluster

	// First pass: cluster with moderate perceptual distance
	for _, entry := range allColors {
		merged := false
		for idx, cl := range clusters {
			if models.ColorDistanceLAB(entry.c, cl.c) < 12.0 {
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
		variations := []models.Color{
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
