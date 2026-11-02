package seeder

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"master-server/internal/games"
	"master-server/internal/registry"
)

func TestSeederMultiFormats(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. JSON Array format
	jsonArrayFile := filepath.Join(tmpDir, "array.json")
	jsonArrayContent := `[
		{"game": "kingpin", "ip": "127.0.0.1", "port": 31510},
		{"game": "quake2", "ip": "127.0.0.2", "port": 27910}
	]`
	if err := os.WriteFile(jsonArrayFile, []byte(jsonArrayContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. JSON Map format
	jsonMapFile := filepath.Join(tmpDir, "map.json")
	jsonMapContent := `{
		"kingpin": ["127.0.0.3:31510"],
		"quake3": ["127.0.0.4:27960"]
	}`
	if err := os.WriteFile(jsonMapFile, []byte(jsonMapContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Text format with directives and inline game
	textFile := filepath.Join(tmpDir, "servers.txt")
	textContent := `# Comment
# @game: ut99
127.0.0.5:7777

[deusex]
127.0.0.6:7791

# Inline game format
rune 127.0.0.7:7778
127.0.0.8:23000 bf1942
`
	if err := os.WriteFile(textFile, []byte(textContent), 0644); err != nil {
		t.Fatal(err)
	}

	gameReg, err := games.NewRegistry()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	store := registry.NewStore(true) // allow private for unit test
	seeder := NewSeeder(store, gameReg, []string{jsonArrayFile, jsonMapFile, textFile}, 1*time.Minute)

	// Fetch all three
	if err := seeder.fetchURL(jsonArrayFile); err != nil {
		t.Fatalf("fetch array: %v", err)
	}
	if err := seeder.fetchURL(jsonMapFile); err != nil {
		t.Fatalf("fetch map: %v", err)
	}
	if err := seeder.fetchURL(textFile); err != nil {
		t.Fatalf("fetch text: %v", err)
	}

	all := store.GetAllForProbing()
	if len(all) < 8 {
		t.Fatalf("expected at least 8 servers imported across all formats, got %d", len(all))
	}

	// Verify Kingpin offset applied: port 31510 -> queryPort 31500
	kpFound := false
	for _, s := range all {
		if s.Gamename == "kingpin" && s.IP == "127.0.0.1" {
			kpFound = true
			if s.GamePort != 31510 {
				t.Errorf("expected Kingpin gamePort 31510, got %d", s.GamePort)
			}
			if s.QueryPort != 31500 {
				t.Errorf("expected Kingpin queryPort 31500, got %d", s.QueryPort)
			}
		}
	}
	if !kpFound {
		t.Error("Kingpin server 127.0.0.1 not found in store")
	}
}

func TestRetroServerMirrorIngestion(t *testing.T) {
	tmpDir := t.TempDir()
	serversSubdir := filepath.Join(tmpDir, "servers")
	if err := os.MkdirAll(serversSubdir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write per-game text files
	kpText := "# Kingpin server list\n127.0.0.10:31510\n127.0.0.11:31512\n"
	if err := os.WriteFile(filepath.Join(serversSubdir, "kingpin.txt"), []byte(kpText), 0644); err != nil {
		t.Fatal(err)
	}

	cod4Text := "# CoD4 server list\n127.0.0.20:28960\n127.0.0.21:28960\n"
	if err := os.WriteFile(filepath.Join(serversSubdir, "cod4.txt"), []byte(cod4Text), 0644); err != nil {
		t.Fatal(err)
	}

	bfText := "# BF1942 server list\n127.0.0.30:14567\n"
	if err := os.WriteFile(filepath.Join(serversSubdir, "bf1942.txt"), []byte(bfText), 0644); err != nil {
		t.Fatal(err)
	}

	// Central servers.json index
	serversJSON := `{
		"updated_at": "2026-11-02T12:00:00Z",
		"total_servers": 5,
		"total_games": 3,
		"active_games_count": 3,
		"games": {
			"kingpin": {
				"name": "Kingpin: Life of Crime",
				"category": "idTech / Quake",
				"protocol": "quake2",
				"server_count": 2,
				"file": "servers/kingpin.txt"
			},
			"cod4": {
				"name": "Call of Duty 4",
				"category": "idTech / Quake",
				"protocol": "quake3",
				"server_count": 2,
				"file": "servers/cod4.txt"
			},
			"bf1942": {
				"name": "Battlefield 1942",
				"category": "EA / DICE",
				"protocol": "gamespy1",
				"server_count": 1,
				"file": "servers/bf1942.txt"
			},
			"inactivegame": {
				"name": "Inactive Game",
				"category": "Other",
				"protocol": "gamespy1",
				"server_count": 0,
				"file": null
			}
		}
	}`
	indexPath := filepath.Join(tmpDir, "servers.json")
	if err := os.WriteFile(indexPath, []byte(serversJSON), 0644); err != nil {
		t.Fatal(err)
	}

	gameReg, err := games.NewRegistry()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	store := registry.NewStore(true)
	seeder := NewSeeder(store, gameReg, []string{indexPath}, 1*time.Minute)

	if err := seeder.fetchURL(indexPath); err != nil {
		t.Fatalf("failed to ingest retro-server-mirror servers.json: %v", err)
	}

	servers := store.GetAllForProbing()
	if len(servers) != 5 {
		t.Fatalf("expected exactly 5 servers registered, got %d", len(servers))
	}

	// Verify BF1942 gameport 14567 -> queryPort 23000
	bfFound := false
	for _, s := range servers {
		if s.Gamename == "bf1942" && s.IP == "127.0.0.30" {
			bfFound = true
			if s.GamePort != 14567 {
				t.Errorf("expected BF1942 gamePort 14567, got %d", s.GamePort)
			}
			if s.QueryPort != 23000 {
				t.Errorf("expected BF1942 queryPort 23000, got %d", s.QueryPort)
			}
		}
	}
	if !bfFound {
		t.Error("BF1942 server not found in store")
	}

	// Verify Kingpin gameport 31510 -> queryPort 31500
	kpFound := false
	for _, s := range servers {
		if s.Gamename == "kingpin" && s.IP == "127.0.0.10" {
			kpFound = true
			if s.GamePort != 31510 {
				t.Errorf("expected Kingpin gamePort 31510, got %d", s.GamePort)
			}
			if s.QueryPort != 31500 {
				t.Errorf("expected Kingpin queryPort 31500, got %d", s.QueryPort)
			}
		}
	}
	if !kpFound {
		t.Error("Kingpin server 127.0.0.10 not found in store")
	}
}
