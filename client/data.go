package client

import "encoding/json"

// GameLanguages is a map of language codes to their full names.
var GameLanguages = map[string]string{
	"en":      "English",
	"fr":      "Français",
	"de":      "Deutsch",
	"es":      "Español",
	"it":      "Italiano",
	"ru":      "Русский",
	"pl":      "Polski",
	"pt-BR":   "Português do Brasil",
	"zh-Hans": "简体中文",
	"ja":      "日本語",
	"ko":      "한국어",
}

// Game contains information about a game and its downloadable content like extras and DLCs.
type Game struct {
	Title           string         `json:"title"`
	BackgroundImage *string        `json:"backgroundImage,omitempty"`
	Downloads       []Downloadable `json:"downloads"`
	Extras          []Extra        `json:"extras"`
	DLCs            []DLC          `json:"dlcs"`
}

// PlatformFile contains information about a platform-specific installation file.
type PlatformFile struct {
	ManualURL *string `json:"manualUrl,omitempty"`
	Name      string  `json:"name"`
	Version   *string `json:"version,omitempty"`
	Date      *string `json:"date,omitempty"`
	Size      string  `json:"size"`
}

// Extra contains information about an extra file like game manual and soundtracks.
type Extra struct {
	Name      string `json:"name"`
	Size      string `json:"size"`
	ManualURL string `json:"manualUrl"`
}

// DLC contains information about a downloadable content like expansions and updates.
type DLC struct {
	Title           string          `json:"title"`
	BackgroundImage *string         `json:"backgroundImage,omitempty"`
	Downloads       [][]interface{} `json:"downloads"`
	Extras          []Extra         `json:"extras"`
	ParsedDownloads []Downloadable  `json:"-"`
}

// Platform contains information about platform-specific installation files.
type Platform struct {
	Windows []PlatformFile `json:"windows,omitempty"`
	Mac     []PlatformFile `json:"mac,omitempty"`
	Linux   []PlatformFile `json:"linux,omitempty"`
}

// Downloadable contains information about a downloadable file for a specific language and platform.
type Downloadable struct {
	Language  string   `json:"language"`
	Platforms Platform `json:"platforms"`
}

// UnmarshalJSON is a custom unmarshal function for Game to process downloads and DLCs correctly.
func (gd *Game) UnmarshalJSON(data []byte) error {
	type Alias Game
	// First, try to unmarshal into a flexible structure that handles API variations
	var raw struct {
		Title           string          `json:"title"`
		BackgroundImage *string         `json:"backgroundImage,omitempty"`
		RawDownloads    json.RawMessage `json:"downloads"`
		Extras          []Extra         `json:"extras"`
		DLCs            []struct {
			Title           string          `json:"title"`
			BackgroundImage *string         `json:"backgroundImage,omitempty"`
			RawDownloads    json.RawMessage `json:"downloads"`
			Extras          []Extra         `json:"extras"`
		} `json:"dlcs"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Copy basic fields
	gd.Title = raw.Title
	gd.BackgroundImage = raw.BackgroundImage
	gd.Extras = raw.Extras

	// Process RawDownloads for Game - handle different formats
	gd.Downloads = parseRawDownloadsFlexible(raw.RawDownloads)

	// Process DLCs
	gd.DLCs = make([]DLC, len(raw.DLCs))
	for i, rawDLC := range raw.DLCs {
		gd.DLCs[i] = DLC{
			Title:           rawDLC.Title,
			BackgroundImage: rawDLC.BackgroundImage,
			Extras:          rawDLC.Extras,
		}
		gd.DLCs[i].ParsedDownloads = parseRawDownloadsFlexible(rawDLC.RawDownloads)
	}

	return nil
}

// parseRawDownloadsFlexible handles different JSON formats for downloads from GOG API.
func parseRawDownloadsFlexible(rawJSON json.RawMessage) []Downloadable {
	if len(rawJSON) == 0 || string(rawJSON) == "null" || string(rawJSON) == "[]" {
		return nil
	}

	// Try format 1: [][]interface{} (expected format)
	var arrArr [][]interface{}
	if err := json.Unmarshal(rawJSON, &arrArr); err == nil {
		return parseRawDownloads(arrArr)
	}

	// Try format 2: []interface{} with objects (alternative format)
	var arr []interface{}
	if err := json.Unmarshal(rawJSON, &arr); err == nil {
		// Check if it's an array of objects with language/platforms keys
		var downloads []Downloadable
		for _, item := range arr {
			if obj, ok := item.(map[string]interface{}); ok {
				// Direct object format: {"language": "en", "windows": [...], ...}
				dl := Downloadable{}
				if lang, ok := obj["language"].(string); ok {
					dl.Language = lang
				}
				// Try to parse platforms from the object
				platformsData, _ := json.Marshal(obj)
				var platforms Platform
				if json.Unmarshal(platformsData, &platforms) == nil {
					dl.Platforms = platforms
				}
				if dl.Language != "" || len(dl.Platforms.Windows) > 0 || len(dl.Platforms.Mac) > 0 || len(dl.Platforms.Linux) > 0 {
					downloads = append(downloads, dl)
				}
			} else if tuple, ok := item.([]interface{}); ok {
				// Tuple format: [language, platforms]
				if len(tuple) == 2 {
					if lang, ok := tuple[0].(string); ok {
						if platforms, err := parsePlatforms(tuple[1]); err == nil {
							downloads = append(downloads, Downloadable{
								Language:  lang,
								Platforms: platforms,
							})
						}
					}
				}
			}
		}
		return downloads
	}

	return nil
}

// parseRawDownloads parses the raw downloads data into a slice of Downloadable.
func parseRawDownloads(rawDownloads [][]interface{}) []Downloadable {
	var downloads []Downloadable

	for _, raw := range rawDownloads {
		if len(raw) != 2 {
			continue
		}

		// First element is the language.
		language, ok := raw[0].(string)
		if !ok {
			continue
		}

		// Second element is the platforms object.
		platforms, err := parsePlatforms(raw[1])
		if err != nil {
			continue
		}

		downloads = append(downloads, Downloadable{
			Language:  language,
			Platforms: platforms,
		})
	}

	return downloads
}

// parsePlatforms parses the platforms data from an interface{}.
func parsePlatforms(data interface{}) (Platform, error) {
	platformsData, err := json.Marshal(data)
	if err != nil {
		return Platform{}, err
	}

	var platforms Platform
	if err := json.Unmarshal(platformsData, &platforms); err != nil {
		return Platform{}, err
	}

	return platforms, nil
}
