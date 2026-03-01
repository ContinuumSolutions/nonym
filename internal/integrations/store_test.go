package integrations

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	key := make([]byte, 32) // all-zero key for tests
	s := NewStore(db, key)
	if err := s.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return s
}

func TestStoreList_EmptyWithoutSeed(t *testing.T) {
	s := newTestStore(t)
	svcs, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(svcs) != 0 {
		t.Errorf("want 0 services without seed, got %d", len(svcs))
	}
}

func TestStoreSeed_InsertsBuiltins(t *testing.T) {
	s := newTestStore(t)
	if err := s.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	svcs, err := s.List()
	if err != nil {
		t.Fatalf("List after Seed: %v", err)
	}
	if len(svcs) == 0 {
		t.Error("want services after Seed, got 0")
	}
}

func TestStoreSeed_Idempotent(t *testing.T) {
	s := newTestStore(t)
	if err := s.Seed(); err != nil {
		t.Fatalf("first Seed: %v", err)
	}
	svcs1, _ := s.List()
	if err := s.Seed(); err != nil {
		t.Fatalf("second Seed: %v", err)
	}
	svcs2, _ := s.List()
	if len(svcs1) != len(svcs2) {
		t.Errorf("Seed not idempotent: first=%d second=%d", len(svcs1), len(svcs2))
	}
}

func TestStoreCreateCustom_Roundtrip(t *testing.T) {
	s := newTestStore(t)
	input := &Service{
		Name:        "My API",
		Category:    "Finance",
		APIEndpoint: "https://api.example.com",
		APIKey:      "supersecret1234",
	}
	svc, err := s.CreateCustom(input)
	if err != nil {
		t.Fatalf("CreateCustom: %v", err)
	}
	if svc.ID == 0 {
		t.Error("want non-zero ID after CreateCustom")
	}
	if svc.Status != Installed {
		t.Errorf("want Installed status, got %v", svc.Status)
	}
	if !svc.Custom {
		t.Error("want Custom=true")
	}
	// API key is stored encrypted and returned masked
	if svc.APIKey != "••••1234" {
		t.Errorf("want masked key ••••1234, got %q", svc.APIKey)
	}
	if svc.APIEndpoint != "https://api.example.com" {
		t.Errorf("APIEndpoint: got %q", svc.APIEndpoint)
	}
}

func TestStoreGet_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Get(99999)
	if err != ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestStoreStartConnect_SetsInProgress(t *testing.T) {
	s := newTestStore(t)
	if err := s.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	svcs, _ := s.List()
	id := svcs[0].ID
	svc, err := s.StartConnect(id)
	if err != nil {
		t.Fatalf("StartConnect: %v", err)
	}
	if svc.Status != InProgress {
		t.Errorf("want InProgress, got %v", svc.Status)
	}
}

func TestStoreStartConnect_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.StartConnect(99999)
	if err != ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestStoreCompleteConnect_APIKey(t *testing.T) {
	s := newTestStore(t)
	if err := s.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	svcs, _ := s.List()
	id := svcs[0].ID
	s.StartConnect(id)

	svc, err := s.CompleteConnect(id, ConnectInput{APIKey: "my-api-key-5678"})
	if err != nil {
		t.Fatalf("CompleteConnect: %v", err)
	}
	if svc.Status != Installed {
		t.Errorf("want Installed, got %v", svc.Status)
	}
	// Key is stored encrypted; returned masked (>4 chars so has suffix)
	if svc.APIKey == "" {
		t.Error("want masked API key, got empty")
	}
}

func TestStoreCompleteConnect_OAuth(t *testing.T) {
	s := newTestStore(t)
	if err := s.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	svcs, _ := s.List()
	id := svcs[0].ID
	s.StartConnect(id)

	svc, err := s.CompleteConnect(id, ConnectInput{OAuthAccessToken: "ya29.access_token"})
	if err != nil {
		t.Fatalf("CompleteConnect OAuth: %v", err)
	}
	if svc.Status != Installed {
		t.Errorf("want Installed, got %v", svc.Status)
	}
	if !svc.OAuthConnected {
		t.Error("want OAuthConnected=true after OAuth token saved")
	}
}

func TestStoreCompleteConnect_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.CompleteConnect(99999, ConnectInput{APIKey: "key"})
	if err != ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestStoreUninstall_ResetsToPending(t *testing.T) {
	s := newTestStore(t)
	svc, err := s.CreateCustom(&Service{
		Name:        "X",
		Category:    "Y",
		APIEndpoint: "https://x.com",
		APIKey:      "key1234",
	})
	if err != nil {
		t.Fatalf("CreateCustom: %v", err)
	}
	uninstalled, err := s.Uninstall(svc.ID)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if uninstalled.Status != Pending {
		t.Errorf("want Pending after Uninstall, got %v", uninstalled.Status)
	}
	if uninstalled.APIKey != "" {
		t.Errorf("want empty APIKey after Uninstall, got %q", uninstalled.APIKey)
	}
}

func TestStoreUninstall_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Uninstall(99999)
	if err != ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestStoreListInstalled_ReturnsOnlyInstalled(t *testing.T) {
	s := newTestStore(t)
	// CreateCustom sets status=Installed immediately
	s.CreateCustom(&Service{Name: "A", Category: "Z", APIEndpoint: "https://a.com", APIKey: "key1234"})
	// Create another then uninstall it
	svc2, _ := s.CreateCustom(&Service{Name: "B", Category: "Z", APIEndpoint: "https://b.com", APIKey: "key5678"})
	s.Uninstall(svc2.ID)

	installed, err := s.ListInstalled()
	if err != nil {
		t.Fatalf("ListInstalled: %v", err)
	}
	if len(installed) != 1 {
		t.Errorf("want 1 installed, got %d", len(installed))
	}
	if installed[0].Name != "A" {
		t.Errorf("want service A, got %q", installed[0].Name)
	}
}

func TestStoreListInstalled_DecryptsCredentials(t *testing.T) {
	s := newTestStore(t)
	s.CreateCustom(&Service{Name: "C", Category: "Z", APIEndpoint: "https://c.com", APIKey: "myrawkey"})

	installed, err := s.ListInstalled()
	if err != nil {
		t.Fatalf("ListInstalled: %v", err)
	}
	if len(installed) != 1 {
		t.Fatalf("want 1 installed, got %d", len(installed))
	}
	// ListInstalled returns the raw (decrypted) key, not the masked version
	if installed[0].APIKey != "myrawkey" {
		t.Errorf("want raw key %q, got %q", "myrawkey", installed[0].APIKey)
	}
}

// ── Color field ───────────────────────────────────────────────────────────────

func TestStoreSeed_ColorPresentOnBuiltins(t *testing.T) {
	s := newTestStore(t)
	if err := s.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	svcs, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, svc := range svcs {
		if svc.Color == "" {
			t.Errorf("service %q has empty color after Seed", svc.Slug)
		}
	}
}

func TestStoreSeed_ColorSyncedOnReSeed(t *testing.T) {
	s := newTestStore(t)
	// Seed once — color column populated
	if err := s.Seed(); err != nil {
		t.Fatalf("first Seed: %v", err)
	}
	// Wipe color to simulate a pre-upgrade database
	s.db.Exec(`UPDATE services SET color = '' WHERE custom = 0`)

	// Re-seed — Seed() must restore colors
	if err := s.Seed(); err != nil {
		t.Fatalf("second Seed: %v", err)
	}
	svcs, _ := s.List()
	for _, svc := range svcs {
		if svc.Color == "" {
			t.Errorf("service %q color not restored on re-seed", svc.Slug)
		}
	}
}

func TestStoreCreateCustom_ColorEmptyByDefault(t *testing.T) {
	s := newTestStore(t)
	svc, err := s.CreateCustom(&Service{
		Name:        "My API",
		Category:    CategoryFinance,
		APIEndpoint: "https://api.example.com",
		APIKey:      "key123",
	})
	if err != nil {
		t.Fatalf("CreateCustom: %v", err)
	}
	// Custom services have no brand color; frontend falls back to category color.
	if svc.Color != "" {
		t.Errorf("want empty color for custom service, got %q", svc.Color)
	}
}

func TestStoreGet_ColorRoundTrips(t *testing.T) {
	s := newTestStore(t)
	s.Seed()
	svcs, _ := s.List()

	// Spot-check a known service
	var gmail *Service
	for i := range svcs {
		if svcs[i].Slug == "gmail" {
			gmail = &svcs[i]
			break
		}
	}
	if gmail == nil {
		t.Fatal("gmail not found after Seed")
	}
	if gmail.Color != "#EA4335" {
		t.Errorf("gmail color: want #EA4335, got %q", gmail.Color)
	}

	// Fetch by ID and confirm color survives the scanRow path
	fetched, err := s.Get(gmail.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fetched.Color != "#EA4335" {
		t.Errorf("Get gmail color: want #EA4335, got %q", fetched.Color)
	}
}
