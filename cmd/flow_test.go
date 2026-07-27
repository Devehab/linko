package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ibtkrgo/linko/cloudflare"
	"github.com/ibtkrgo/linko/config"
)

// fakeAPI is a very small stand-in for the Cloudflare API: it keeps a tunnel
// ingress list and a DNS record table in memory.
type fakeAPI struct {
	ingress []cloudflare.IngressRule
	records map[string]cloudflare.DNSRecord
	deleted []string
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{records: map[string]cloudflare.DNSRecord{}}
}

func (f *fakeAPI) server(t *testing.T) (*cloudflare.Client, *config.Config) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var result any = map[string]any{}

		switch {
		case strings.HasSuffix(r.URL.Path, "/configurations") && r.Method == http.MethodGet:
			result = map[string]any{"tunnel_id": "tun-1", "config": map[string]any{"ingress": f.ingress}}

		case strings.HasSuffix(r.URL.Path, "/configurations") && r.Method == http.MethodPut:
			var body struct {
				Config cloudflare.TunnelConfig `json:"config"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.ingress = body.Config.Ingress

		case strings.Contains(r.URL.Path, "/dns_records") && r.Method == http.MethodGet:
			name := r.URL.Query().Get("name")
			out := []cloudflare.DNSRecord{}
			if rec, ok := f.records[name]; ok {
				out = append(out, rec)
			}
			result = out

		case strings.Contains(r.URL.Path, "/dns_records") && r.Method == http.MethodDelete:
			parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			id := parts[len(parts)-1]
			f.deleted = append(f.deleted, id)
			for name, rec := range f.records {
				if rec.ID == id {
					delete(f.records, name)
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"errors":  []any{},
			"result":  result,
		})
	}))
	t.Cleanup(srv.Close)

	client := cloudflare.New("t")
	client.BaseURL = srv.URL
	client.AccountID = "acc-1"
	client.ZoneID = "zone-1"

	cfg := &config.Config{
		AccountID:  "acc-1",
		ZoneID:     "zone-1",
		Domain:     "example.com",
		BaseDomain: "demo.example.com",
		TunnelID:   "tun-1",
		TunnelName: "example-linko-tunnel",
	}
	return client, cfg
}

func TestRemoveRouteCleansUpEverything(t *testing.T) {
	api := newFakeAPI()
	api.ingress = cloudflare.UpsertIngress(nil, "crm.demo.example.com", "http://localhost:3000")
	api.ingress = cloudflare.UpsertIngress(api.ingress, "api.demo.example.com", "http://localhost:8080")
	api.records["crm.demo.example.com"] = cloudflare.DNSRecord{
		ID: "rec-crm", Type: "CNAME", Name: "crm.demo.example.com", Content: "tun-1.cfargotunnel.com",
	}

	client, cfg := api.server(t)
	route := config.Route{Name: "crm", Hostname: "crm.demo.example.com", Service: "http://localhost:3000", DNSRecordID: "rec-crm"}
	cfg.UpsertRoute(route)

	if err := removeRoute(context.Background(), client, cfg, route); err != nil {
		t.Fatalf("removeRoute() = %v", err)
	}

	if _, ok := cloudflare.FindIngress(api.ingress, "crm.demo.example.com"); ok {
		t.Error("the ingress rule was not removed")
	}
	if _, ok := cloudflare.FindIngress(api.ingress, "api.demo.example.com"); !ok {
		t.Error("removeRoute removed an unrelated route")
	}
	if last := api.ingress[len(api.ingress)-1]; last.Hostname != "" || last.Service != cloudflare.CatchAllService {
		t.Errorf("the catch-all rule is missing after removal: %+v", api.ingress)
	}
	if len(api.deleted) != 1 || api.deleted[0] != "rec-crm" {
		t.Errorf("DNS deletions = %v, want [rec-crm]", api.deleted)
	}
	if cfg.FindRoute("crm") != nil {
		t.Error("the route is still in the local config")
	}
}

func TestRemoveRouteLooksUpMissingRecordID(t *testing.T) {
	api := newFakeAPI()
	api.ingress = cloudflare.UpsertIngress(nil, "crm.demo.example.com", "http://localhost:3000")
	api.records["crm.demo.example.com"] = cloudflare.DNSRecord{
		ID: "rec-found", Type: "CNAME", Name: "crm.demo.example.com", Content: "tun-1.cfargotunnel.com",
	}

	client, cfg := api.server(t)
	route := config.Route{Name: "crm", Hostname: "crm.demo.example.com"} // no DNSRecordID

	if err := removeRoute(context.Background(), client, cfg, route); err != nil {
		t.Fatalf("removeRoute() = %v", err)
	}
	if len(api.deleted) != 1 || api.deleted[0] != "rec-found" {
		t.Fatalf("DNS deletions = %v, want [rec-found]", api.deleted)
	}
}

func TestRemoveRouteSkipsForeignDNSRecords(t *testing.T) {
	api := newFakeAPI()
	api.ingress = cloudflare.UpsertIngress(nil, "crm.demo.example.com", "http://localhost:3000")
	// A record that is not owned by a tunnel must not be deleted.
	api.records["crm.demo.example.com"] = cloudflare.DNSRecord{
		ID: "rec-website", Type: "CNAME", Name: "crm.demo.example.com", Content: "some-host.example.net",
	}

	client, cfg := api.server(t)
	route := config.Route{Name: "crm", Hostname: "crm.demo.example.com"}

	if err := removeRoute(context.Background(), client, cfg, route); err != nil {
		t.Fatalf("removeRoute() = %v", err)
	}
	if len(api.deleted) != 0 {
		t.Fatalf("deleted %v, want nothing — the record is not a tunnel CNAME", api.deleted)
	}
}

func TestRemoveRouteIsIdempotent(t *testing.T) {
	api := newFakeAPI()
	api.ingress = cloudflare.NormalizeIngress(nil)

	client, cfg := api.server(t)
	route := config.Route{Name: "gone", Hostname: "gone.demo.example.com"}

	if err := removeRoute(context.Background(), client, cfg, route); err != nil {
		t.Fatalf("removeRoute() on a missing route = %v", err)
	}
	if len(api.ingress) != 1 {
		t.Fatalf("ingress = %+v, want just the catch-all", api.ingress)
	}
}
