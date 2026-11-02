package registry

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStorePersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "store_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "sub", "servers.json")

	// 1. Initial Store
	s1 := NewStore(true)

	// Loading non-existent file should be a clean no-op
	loaded, err := s1.LoadFromFile(dbPath)
	if err != nil {
		t.Fatalf("expected nil error on missing file, got %v", err)
	}
	if loaded != 0 {
		t.Fatalf("expected 0 loaded servers, got %d", loaded)
	}

	// 2. Add sample servers
	s1.RegisterHeartbeat("1.2.3.4", 27960, "quake3")
	s1.UpdateProbeResult(
		"1.2.3.4",
		27960,
		27960,
		"Test Q3 Arena Server",
		"q3dm6",
		8,
		16,
		42,
		[]Player{{Name: "Ranger", Frags: 12, Ping: 35}},
		map[string]string{"fraglimit": "20", "timelimit": "15"},
	)

	s1.RegisterHeartbeat("5.6.7.8", 31510, "kingpin")
	s1.UpdateProbeResult(
		"5.6.7.8",
		31500, // normalized queryport
		31510,
		"Kingpin Crime Alley",
		"kpm01",
		4,
		12,
		68,
		[]Player{{Name: "Louie", Frags: 5, Ping: 65}},
		map[string]string{"deathmatch": "1"},
	)

	if !s1.IsDirty() {
		t.Fatalf("expected store to be dirty after updates")
	}

	// 3. Save to file
	if err := s1.SaveToFile(dbPath); err != nil {
		t.Fatalf("failed to save store to file: %v", err)
	}

	if s1.IsDirty() {
		t.Fatalf("expected store dirty to be cleared after save")
	}

	// Verify file was written
	stat, err := os.Stat(dbPath)
	if err != nil || stat.Size() == 0 {
		t.Fatalf("db file was not created or is empty: %v", err)
	}

	// 4. Create a fresh Store s2 and load from DB
	s2 := NewStore(true)
	loaded, err = s2.LoadFromFile(dbPath)
	if err != nil {
		t.Fatalf("failed to load store from file: %v", err)
	}
	if loaded != 2 {
		t.Fatalf("expected 2 servers loaded, got %d", loaded)
	}

	// 5. Verify snapshot in s2
	snap := s2.GetSnapshot()
	if snap.TotalServers != 2 {
		t.Fatalf("expected 2 total verified servers in snapshot, got %d", snap.TotalServers)
	}
	if snap.TotalPlayers != 12 { // 8 + 4
		t.Fatalf("expected 12 total players, got %d", snap.TotalPlayers)
	}

	// Verify Q3 server fields
	q3Servers := s2.GetServersByGame("quake3")
	if len(q3Servers) != 1 {
		t.Fatalf("expected 1 quake3 server, got %d", len(q3Servers))
	}
	q3 := q3Servers[0]
	if q3.Hostname != "Test Q3 Arena Server" {
		t.Errorf("expected hostname 'Test Q3 Arena Server', got '%s'", q3.Hostname)
	}
	if q3.Ping != 42 {
		t.Errorf("expected ping 42, got %d", q3.Ping)
	}
	if len(q3.Roster) != 1 || q3.Roster[0].Name != "Ranger" {
		t.Errorf("expected roster player 'Ranger', got %+v", q3.Roster)
	}
	if q3.CVars["fraglimit"] != "20" {
		t.Errorf("expected fraglimit '20', got '%s'", q3.CVars["fraglimit"])
	}
}

func TestStoreAutoSave(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "store_autosave_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "autosave.json")

	store := NewStore(true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go store.StartAutoSave(ctx, 50*time.Millisecond, dbPath)

	// Initially not dirty, file shouldn't exist
	time.Sleep(100 * time.Millisecond)
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("expected file to not exist before changes")
	}

	// Mutate store
	store.RegisterHeartbeat("9.9.9.9", 27910, "quake2")

	// Wait for auto-save ticker
	time.Sleep(150 * time.Millisecond)

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected file to exist after auto-save: %v", err)
	}

	if store.IsDirty() {
		t.Fatalf("expected store to not be dirty after auto-save")
	}
}
