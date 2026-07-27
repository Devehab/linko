package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(EnvHome, dir)
	t.Setenv(EnvToken, "")
	return dir
}

func TestDirHonoursEnv(t *testing.T) {
	dir := withTempHome(t)
	if got := Dir(); got != dir {
		t.Fatalf("Dir() = %q, want %q", got, dir)
	}
	if got, want := Path(), filepath.Join(dir, fileName); got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
	if got, want := BinDir(), filepath.Join(dir, "bin"); got != want {
		t.Fatalf("BinDir() = %q, want %q", got, want)
	}
}

func TestLoadWithoutConfig(t *testing.T) {
	withTempHome(t)
	if Exists() {
		t.Fatal("Exists() = true for an empty home")
	}
	if _, err := Load(); err != ErrNotInitialized {
		t.Fatalf("Load() error = %v, want ErrNotInitialized", err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	withTempHome(t)

	want := &Config{
		APIToken:   "token-123",
		AccountID:  "acc",
		ZoneID:     "zone",
		Domain:     "example.com",
		BaseDomain: "demo.example.com",
		TunnelID:   "tun",
		TunnelName: "example-linko-tunnel",
		Routes: []Route{{
			Name:      "crm",
			Hostname:  "crm.demo.example.com",
			Service:   "http://localhost:3000",
			Port:      3000,
			CreatedAt: time.Now().UTC().Truncate(time.Second),
		}},
	}
	if err := want.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if !Exists() {
		t.Fatal("Exists() = false after Save()")
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.APIToken != want.APIToken || got.TunnelID != want.TunnelID || got.BaseDomain != want.BaseDomain {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if len(got.Routes) != 1 || got.Routes[0].Name != "crm" || got.Routes[0].Port != 3000 {
		t.Fatalf("routes not preserved: %+v", got.Routes)
	}
}

func TestSaveUsesRestrictivePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes differ on windows")
	}
	withTempHome(t)

	c := &Config{APIToken: "secret"}
	if err := c.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	st, err := os.Stat(Path())
	if err != nil {
		t.Fatal(err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config permissions = %v, want 0600", perm)
	}

	// Saving twice must keep the permissions.
	if err := c.Save(); err != nil {
		t.Fatalf("second Save() error = %v", err)
	}
	st, _ = os.Stat(Path())
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config permissions after rewrite = %v, want 0600", perm)
	}
}

func TestEnvTokenOverridesStoredToken(t *testing.T) {
	withTempHome(t)
	c := &Config{APIToken: "stored"}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvToken, "from-env")

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.APIToken != "from-env" {
		t.Fatalf("APIToken = %q, want %q", got.APIToken, "from-env")
	}
}

func TestLoadCorruptFile(t *testing.T) {
	dir := withTempHome(t)
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want a parse error")
	}
}

func TestHostname(t *testing.T) {
	c := &Config{BaseDomain: "demo.example.com"}
	cases := map[string]string{
		"crm":                  "crm.demo.example.com",
		"CRM":                  "crm.demo.example.com",
		"":                     "demo.example.com",
		"@":                    "demo.example.com",
		"crm.demo.example.com": "crm.demo.example.com",
		"demo.example.com":     "demo.example.com",
	}
	for in, want := range cases {
		if got := c.Hostname(in); got != want {
			t.Errorf("Hostname(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUpsertAndRemoveRoute(t *testing.T) {
	c := &Config{BaseDomain: "demo.example.com"}

	c.UpsertRoute(Route{Name: "crm", Hostname: "crm.demo.example.com", Service: "http://localhost:3000"})
	if len(c.Routes) != 1 {
		t.Fatalf("len(Routes) = %d, want 1", len(c.Routes))
	}
	created := c.Routes[0].CreatedAt
	if created.IsZero() {
		t.Fatal("CreatedAt was not set")
	}

	// Replacing the same hostname must not duplicate or reset CreatedAt.
	c.UpsertRoute(Route{Name: "crm", Hostname: "crm.demo.example.com", Service: "http://localhost:5000"})
	if len(c.Routes) != 1 {
		t.Fatalf("len(Routes) = %d after replace, want 1", len(c.Routes))
	}
	if c.Routes[0].Service != "http://localhost:5000" {
		t.Fatalf("Service = %q, want the new value", c.Routes[0].Service)
	}
	if !c.Routes[0].CreatedAt.Equal(created) {
		t.Fatal("CreatedAt was reset on replace")
	}

	c.UpsertRoute(Route{Name: "api", Hostname: "api.demo.example.com", Service: "http://localhost:8080"})
	if len(c.Routes) != 2 {
		t.Fatalf("len(Routes) = %d, want 2", len(c.Routes))
	}

	if c.FindRoute("API") == nil {
		t.Fatal("FindRoute is not case-insensitive")
	}
	if c.FindRouteByHostname("api.demo.example.com") == nil {
		t.Fatal("FindRouteByHostname returned nil")
	}
	if c.FindRoute("nope") != nil {
		t.Fatal("FindRoute found a route that does not exist")
	}

	sorted := c.SortedRoutes()
	if sorted[0].Name != "api" || sorted[1].Name != "crm" {
		t.Fatalf("SortedRoutes not sorted: %v", sorted)
	}

	removed, ok := c.RemoveRoute("crm")
	if !ok || removed.Name != "crm" {
		t.Fatalf("RemoveRoute = %+v, %v", removed, ok)
	}
	if len(c.Routes) != 1 {
		t.Fatalf("len(Routes) = %d after remove, want 1", len(c.Routes))
	}
	if _, ok := c.RemoveRoute("crm"); ok {
		t.Fatal("RemoveRoute removed the same route twice")
	}
	if _, ok := c.RemoveRoute("api.demo.example.com"); !ok {
		t.Fatal("RemoveRoute did not match on hostname")
	}
}

func TestValidate(t *testing.T) {
	full := Config{
		APIToken:   "t",
		AccountID:  "a",
		ZoneID:     "z",
		BaseDomain: "demo.example.com",
		TunnelID:   "tu",
	}
	if err := full.Validate(); err != nil {
		t.Fatalf("Validate() on a complete config = %v", err)
	}

	blanks := map[string]func(*Config){
		"token":   func(c *Config) { c.APIToken = "" },
		"account": func(c *Config) { c.AccountID = "" },
		"zone":    func(c *Config) { c.ZoneID = "" },
		"base":    func(c *Config) { c.BaseDomain = "" },
		"tunnel":  func(c *Config) { c.TunnelID = "" },
	}
	for name, blank := range blanks {
		c := full
		blank(&c)
		if err := c.Validate(); err == nil {
			t.Errorf("Validate() with a missing %s = nil, want an error", name)
		}
	}
}
