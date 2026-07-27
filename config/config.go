// Package config manages linko's on-disk state (~/.linko/config.json).
//
// The file holds the Cloudflare credentials and the list of hostnames the user
// has published. It is written atomically with 0600 permissions because it
// contains an API token.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	dirName  = ".linko"
	fileName = "config.json"

	// EnvHome overrides the configuration directory (mainly for tests).
	EnvHome = "LINKO_HOME"
	// EnvToken overrides the stored Cloudflare API token.
	EnvToken = "LINKO_API_TOKEN"
)

// ErrNotInitialized is returned when no config file exists yet.
var ErrNotInitialized = errors.New("linko is not set up yet — run `linko init` first")

// Route is a published hostname mapped to a local service.
type Route struct {
	Name        string    `json:"name"`
	Hostname    string    `json:"hostname"`
	Service     string    `json:"service"`
	Port        int       `json:"port,omitempty"`
	DNSRecordID string    `json:"dns_record_id,omitempty"`
	Ephemeral   bool      `json:"ephemeral,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Config is the full contents of ~/.linko/config.json.
type Config struct {
	APIToken    string  `json:"api_token"`
	AccountID   string  `json:"account_id"`
	AccountName string  `json:"account_name,omitempty"`
	ZoneID      string  `json:"zone_id"`
	Domain      string  `json:"domain"`
	BaseDomain  string  `json:"base_domain"`
	TunnelID    string  `json:"tunnel_id"`
	TunnelName  string  `json:"tunnel_name"`
	TunnelToken string  `json:"tunnel_token"`
	Routes      []Route `json:"routes"`
}

// Dir returns the linko configuration directory.
func Dir() string {
	if v := strings.TrimSpace(os.Getenv(EnvHome)); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return dirName
	}
	return filepath.Join(home, dirName)
}

// Path returns the full path of the config file.
func Path() string { return filepath.Join(Dir(), fileName) }

// BinDir is where linko keeps a private copy of cloudflared.
func BinDir() string { return filepath.Join(Dir(), "bin") }

// Exists reports whether a config file is present.
func Exists() bool {
	st, err := os.Stat(Path())
	return err == nil && !st.IsDir()
}

// Load reads the config file. It returns ErrNotInitialized if none exists.
func Load() (*Config, error) {
	b, err := os.ReadFile(Path())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotInitialized
		}
		return nil, fmt.Errorf("reading %s: %w", Path(), err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", Path(), err)
	}
	if v := strings.TrimSpace(os.Getenv(EnvToken)); v != "" {
		c.APIToken = v
	}
	return &c, nil
}

// Save writes the config atomically with 0600 permissions.
func (c *Config) Save() error {
	dir := Dir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	tmp, err := os.CreateTemp(dir, ".config-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds

	// Chmod is a no-op on some platforms (Windows); ignore its error there and
	// re-assert the mode after the rename below.
	_ = tmp.Chmod(0o600)

	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, Path()); err != nil {
		return fmt.Errorf("writing %s: %w", Path(), err)
	}
	// Re-assert permissions in case the file already existed.
	_ = os.Chmod(Path(), 0o600)
	return nil
}

// Validate checks that the config has everything the daily commands need.
func (c *Config) Validate() error {
	switch {
	case c == nil:
		return ErrNotInitialized
	case strings.TrimSpace(c.APIToken) == "":
		return errors.New("no Cloudflare API token stored — run `linko init`")
	case strings.TrimSpace(c.AccountID) == "":
		return errors.New("no Cloudflare account id stored — run `linko init`")
	case strings.TrimSpace(c.ZoneID) == "":
		return errors.New("no Cloudflare zone id stored — run `linko init`")
	case strings.TrimSpace(c.BaseDomain) == "":
		return errors.New("no base domain stored — run `linko init`")
	case strings.TrimSpace(c.TunnelID) == "":
		return errors.New("no tunnel stored — run `linko init`")
	}
	return nil
}

// Hostname builds the full hostname for a subdomain label.
func (c *Config) Hostname(label string) string {
	label = strings.TrimSpace(strings.ToLower(label))
	if c.BaseDomain == "" {
		return label
	}
	if label == "" || label == "@" {
		return c.BaseDomain
	}
	if strings.HasSuffix(label, "."+c.BaseDomain) || label == c.BaseDomain {
		return label
	}
	return label + "." + c.BaseDomain
}

// FindRoute returns the route with the given name, or nil.
func (c *Config) FindRoute(name string) *Route {
	for i := range c.Routes {
		if strings.EqualFold(c.Routes[i].Name, name) {
			return &c.Routes[i]
		}
	}
	return nil
}

// FindRouteByHostname returns the route serving a hostname, or nil.
func (c *Config) FindRouteByHostname(hostname string) *Route {
	for i := range c.Routes {
		if strings.EqualFold(c.Routes[i].Hostname, hostname) {
			return &c.Routes[i]
		}
	}
	return nil
}

// UpsertRoute inserts or replaces a route, matching on hostname.
func (c *Config) UpsertRoute(r Route) {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	for i := range c.Routes {
		if strings.EqualFold(c.Routes[i].Hostname, r.Hostname) {
			r.CreatedAt = c.Routes[i].CreatedAt
			c.Routes[i] = r
			return
		}
	}
	c.Routes = append(c.Routes, r)
}

// RemoveRoute deletes a route by name or hostname and reports whether it existed.
func (c *Config) RemoveRoute(nameOrHost string) (Route, bool) {
	for i := range c.Routes {
		if strings.EqualFold(c.Routes[i].Name, nameOrHost) ||
			strings.EqualFold(c.Routes[i].Hostname, nameOrHost) {
			removed := c.Routes[i]
			c.Routes = append(c.Routes[:i], c.Routes[i+1:]...)
			return removed, true
		}
	}
	return Route{}, false
}

// SortedRoutes returns the routes ordered by name.
func (c *Config) SortedRoutes() []Route {
	out := make([]Route, len(c.Routes))
	copy(out, c.Routes)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
