package client

import (
	"encoding/json"
	"testing"
)

func TestGameUnmarshalJSON_StandardFormat(t *testing.T) {
	// Standard GOG response format
	jsonData := `{
		"title": "Test Game",
		"backgroundImage": "//images.gog.com/test.jpg",
		"downloads": [
			["en", {"windows": [{"name": "setup.exe", "size": "1 GB"}]}]
		],
		"extras": [],
		"dlcs": []
	}`

	var game Game
	err := json.Unmarshal([]byte(jsonData), &game)
	if err != nil {
		t.Fatalf("Failed to unmarshal standard format: %v", err)
	}
	if game.Title != "Test Game" {
		t.Errorf("Expected title 'Test Game', got '%s'", game.Title)
	}
	if len(game.Downloads) != 1 {
		t.Errorf("Expected 1 download, got %d", len(game.Downloads))
	}
}

func TestGameUnmarshalJSON_EmptyDownloads(t *testing.T) {
	jsonData := `{
		"title": "Test Game",
		"downloads": [],
		"extras": [],
		"dlcs": []
	}`

	var game Game
	err := json.Unmarshal([]byte(jsonData), &game)
	if err != nil {
		t.Fatalf("Failed to unmarshal empty downloads: %v", err)
	}
}

func TestGameUnmarshalJSON_NullDownloads(t *testing.T) {
	jsonData := `{
		"title": "Test Game",
		"downloads": null,
		"extras": [],
		"dlcs": []
	}`

	var game Game
	err := json.Unmarshal([]byte(jsonData), &game)
	if err != nil {
		t.Fatalf("Failed to unmarshal null downloads: %v", err)
	}
}

func TestGameUnmarshalJSON_ArrayResponse(t *testing.T) {
	// Some GOG endpoints return an array wrapper
	jsonData := `[{
		"title": "Test Game",
		"backgroundImage": "//images.gog.com/test.jpg",
		"downloads": [],
		"extras": [],
		"dlcs": []
	}]`

	// First try to unmarshal as array
	var games []Game
	err := json.Unmarshal([]byte(jsonData), &games)
	if err != nil {
		t.Fatalf("Failed to unmarshal array response: %v", err)
	}
	if len(games) != 1 {
		t.Fatalf("Expected 1 game in array, got %d", len(games))
	}
	if games[0].Title != "Test Game" {
		t.Errorf("Expected title 'Test Game', got '%s'", games[0].Title)
	}
}

func TestGameUnmarshalJSON_ObjectDownloads(t *testing.T) {
	// Alternative format with object-style downloads
	jsonData := `{
		"title": "Test Game",
		"downloads": [
			{"language": "en", "windows": [{"name": "setup.exe", "size": "1 GB"}]}
		],
		"extras": [],
		"dlcs": []
	}`

	var game Game
	err := json.Unmarshal([]byte(jsonData), &game)
	if err != nil {
		t.Fatalf("Failed to unmarshal object downloads: %v", err)
	}
}

func TestParseGameData_Object(t *testing.T) {
	jsonData := []byte(`{
		"title": "Test Game",
		"downloads": [],
		"extras": [],
		"dlcs": []
	}`)

	var game Game
	err := parseGameData(jsonData, &game)
	if err != nil {
		t.Fatalf("Failed to parse object response: %v", err)
	}
	if game.Title != "Test Game" {
		t.Errorf("Expected title 'Test Game', got '%s'", game.Title)
	}
}

func TestParseGameData_Array(t *testing.T) {
	// GOG sometimes returns games as an array with single element
	jsonData := []byte(`[{
		"title": "Array Game",
		"downloads": [],
		"extras": [],
		"dlcs": []
	}]`)

	var game Game
	err := parseGameData(jsonData, &game)
	if err != nil {
		t.Fatalf("Failed to parse array response: %v", err)
	}
	if game.Title != "Array Game" {
		t.Errorf("Expected title 'Array Game', got '%s'", game.Title)
	}
}
