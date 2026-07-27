package cloudflare

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testServer spins up a fake Cloudflare API. handler receives the request and
// returns the JSON body of the `result` field.
func testServer(t *testing.T, handler func(r *http.Request) (any, int)) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result, status := handler(r)
		if status == 0 {
			status = http.StatusOK
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		body := map[string]any{
			"success":  status < 400,
			"errors":   []any{},
			"messages": []any{},
			"result":   result,
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)

	c := New("test-token")
	c.BaseURL = srv.URL
	c.AccountID = "acc-1"
	c.ZoneID = "zone-1"
	return c, srv
}

func TestRequestHeaders(t *testing.T) {
	var gotAuth, gotAccept, gotUA string
	c, _ := testServer(t, func(r *http.Request) (any, int) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotUA = r.Header.Get("User-Agent")
		return map[string]any{"id": "x", "status": "active"}, 200
	})

	if _, err := c.VerifyToken(context.Background()); err != nil {
		t.Fatalf("VerifyToken() = %v", err)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotAccept != "application/json" {
		t.Fatalf("Accept = %q", gotAccept)
	}
	if gotUA == "" {
		t.Fatal("User-Agent was not sent")
	}
}

func TestVerifyTokenFallsBackToZones(t *testing.T) {
	var paths []string
	c, _ := testServer(t, func(r *http.Request) (any, int) {
		paths = append(paths, r.URL.Path)
		switch {
		case strings.HasSuffix(r.URL.Path, "/user/tokens/verify"):
			return nil, http.StatusForbidden
		case strings.Contains(r.URL.Path, "/tokens/verify"):
			return nil, http.StatusForbidden
		case strings.HasSuffix(r.URL.Path, "/zones"):
			return []map[string]any{{"id": "zone-1", "name": "example.com"}}, 200
		}
		return nil, http.StatusNotFound
	})

	ts, err := c.VerifyToken(context.Background())
	if err != nil {
		t.Fatalf("VerifyToken() = %v", err)
	}
	if ts.Status != "active" {
		t.Fatalf("status = %q", ts.Status)
	}
	if len(paths) < 3 {
		t.Fatalf("expected a fallback chain, got %v", paths)
	}
}

func TestAPIErrorSurfacesMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `{"success":false,"errors":[{"code":9109,"message":"Unauthorized to access requested resource"}],"result":null}`)
	}))
	defer srv.Close()

	c := New("bad")
	c.BaseURL = srv.URL
	c.AccountID = "acc-1"

	_, err := c.ListTunnels(context.Background(), "")
	if err == nil {
		t.Fatal("expected an error")
	}
	apiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("error type = %T, want *Error", err)
	}
	if !apiErr.IsAuth() {
		t.Fatal("IsAuth() = false for a 9109 error")
	}
	if !strings.Contains(apiErr.Error(), "Unauthorized to access requested resource") {
		t.Fatalf("message = %q", apiErr.Error())
	}
}

func TestFindZone(t *testing.T) {
	c, _ := testServer(t, func(r *http.Request) (any, int) {
		if got := r.URL.Query().Get("name"); got != "example.com" {
			t.Errorf("name query = %q", got)
		}
		return []map[string]any{{
			"id":      "zone-1",
			"name":    "example.com",
			"account": map[string]any{"id": "acc-9", "name": "Acme"},
		}}, 200
	})

	zone, err := c.FindZone(context.Background(), "Example.COM")
	if err != nil {
		t.Fatalf("FindZone() = %v", err)
	}
	if zone.ID != "zone-1" || zone.Account.ID != "acc-9" || zone.Account.Name != "Acme" {
		t.Fatalf("zone = %+v", zone)
	}
}

func TestFindZoneNotFound(t *testing.T) {
	c, _ := testServer(t, func(r *http.Request) (any, int) {
		return []map[string]any{}, 200
	})
	if _, err := c.FindZone(context.Background(), "missing.com"); err == nil {
		t.Fatal("expected an error for an unknown zone")
	}
}

func TestCreateTunnelSendsConfigSrc(t *testing.T) {
	var body map[string]any
	c, _ := testServer(t, func(r *http.Request) (any, int) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		return map[string]any{"id": "tun-1", "name": body["name"]}, 200
	})

	tun, err := c.CreateTunnel(context.Background(), "my-tunnel")
	if err != nil {
		t.Fatalf("CreateTunnel() = %v", err)
	}
	if tun.ID != "tun-1" || tun.Name != "my-tunnel" {
		t.Fatalf("tunnel = %+v", tun)
	}
	if body["config_src"] != "cloudflare" {
		t.Fatalf("config_src = %v, want cloudflare (remotely managed)", body["config_src"])
	}
}

func TestFindTunnelReturnsNilWhenMissing(t *testing.T) {
	c, _ := testServer(t, func(r *http.Request) (any, int) {
		return []map[string]any{}, 200
	})
	tun, err := c.FindTunnel(context.Background(), "nope")
	if err != nil {
		t.Fatalf("FindTunnel() = %v", err)
	}
	if tun != nil {
		t.Fatalf("tunnel = %+v, want nil", tun)
	}
}

func TestTunnelToken(t *testing.T) {
	c, _ := testServer(t, func(r *http.Request) (any, int) {
		return "eyJhIjoiMSJ9", 200
	})
	token, err := c.TunnelToken(context.Background(), "tun-1")
	if err != nil {
		t.Fatalf("TunnelToken() = %v", err)
	}
	if token != "eyJhIjoiMSJ9" {
		t.Fatalf("token = %q", token)
	}
}

func TestGetTunnelConfigHandlesNullConfig(t *testing.T) {
	c, _ := testServer(t, func(r *http.Request) (any, int) {
		return map[string]any{"tunnel_id": "tun-1", "config": nil, "source": "cloudflare"}, 200
	})
	cfg, err := c.GetTunnelConfig(context.Background(), "tun-1")
	if err != nil {
		t.Fatalf("GetTunnelConfig() = %v", err)
	}
	if cfg == nil || len(cfg.Ingress) != 0 {
		t.Fatalf("config = %+v, want an empty config", cfg)
	}
}

func TestPutTunnelConfigNormalizesIngress(t *testing.T) {
	var sent struct {
		Config TunnelConfig `json:"config"`
	}
	c, _ := testServer(t, func(r *http.Request) (any, int) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&sent)
		return map[string]any{}, 200
	})

	err := c.PutTunnelConfig(context.Background(), "tun-1", &TunnelConfig{
		Ingress: []IngressRule{{Hostname: "a.example.com", Service: "http://localhost:1"}},
	})
	if err != nil {
		t.Fatalf("PutTunnelConfig() = %v", err)
	}
	if n := len(sent.Config.Ingress); n != 2 {
		t.Fatalf("ingress sent = %d rules, want 2 (hostname + catch-all)", n)
	}
	if last := sent.Config.Ingress[1]; last.Hostname != "" || last.Service != CatchAllService {
		t.Fatalf("last rule = %+v, want the catch-all", last)
	}
}

func TestEnsureCNAMECreatesWhenMissing(t *testing.T) {
	created := false
	c, _ := testServer(t, func(r *http.Request) (any, int) {
		if r.Method == http.MethodGet {
			return []map[string]any{}, 200
		}
		created = true
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload["proxied"] != true {
			t.Errorf("proxied = %v, want true", payload["proxied"])
		}
		if payload["type"] != "CNAME" {
			t.Errorf("type = %v", payload["type"])
		}
		return map[string]any{"id": "rec-1", "type": "CNAME", "name": payload["name"], "content": payload["content"], "proxied": true}, 200
	})

	rec, isNew, err := c.EnsureCNAME(context.Background(), "crm.demo.example.com", "tun-1.cfargotunnel.com")
	if err != nil {
		t.Fatalf("EnsureCNAME() = %v", err)
	}
	if !created || !isNew || rec.ID != "rec-1" {
		t.Fatalf("created=%v isNew=%v rec=%+v", created, isNew, rec)
	}
}

func TestEnsureCNAMEIsIdempotent(t *testing.T) {
	writes := 0
	c, _ := testServer(t, func(r *http.Request) (any, int) {
		if r.Method != http.MethodGet {
			writes++
		}
		return []map[string]any{{
			"id":      "rec-1",
			"type":    "CNAME",
			"name":    "crm.demo.example.com",
			"content": "tun-1.cfargotunnel.com",
			"proxied": true,
		}}, 200
	})

	_, isNew, err := c.EnsureCNAME(context.Background(), "crm.demo.example.com", "tun-1.cfargotunnel.com")
	if err != nil {
		t.Fatalf("EnsureCNAME() = %v", err)
	}
	if isNew {
		t.Fatal("isNew = true for an unchanged record")
	}
	if writes != 0 {
		t.Fatalf("%d write(s) issued for an unchanged record", writes)
	}
}

func TestEnsureCNAMERefusesForeignRecords(t *testing.T) {
	c, _ := testServer(t, func(r *http.Request) (any, int) {
		return []map[string]any{{
			"id":      "rec-1",
			"type":    "A",
			"name":    "crm.demo.example.com",
			"content": "203.0.113.10",
		}}, 200
	})
	if _, _, err := c.EnsureCNAME(context.Background(), "crm.demo.example.com", "tun-1.cfargotunnel.com"); err == nil {
		t.Fatal("expected an error when the hostname is already an A record")
	}
}

func TestEnsureCNAMERepointsTunnelRecord(t *testing.T) {
	updated := false
	c, _ := testServer(t, func(r *http.Request) (any, int) {
		if r.Method == http.MethodGet {
			return []map[string]any{{
				"id":      "rec-1",
				"type":    "CNAME",
				"name":    "crm.demo.example.com",
				"content": "old-tunnel.cfargotunnel.com",
				"proxied": true,
			}}, 200
		}
		if r.Method == http.MethodPut {
			updated = true
		}
		return map[string]any{"id": "rec-1", "content": "tun-1.cfargotunnel.com"}, 200
	})

	_, isNew, err := c.EnsureCNAME(context.Background(), "crm.demo.example.com", "tun-1.cfargotunnel.com")
	if err != nil {
		t.Fatalf("EnsureCNAME() = %v", err)
	}
	if isNew || !updated {
		t.Fatalf("isNew=%v updated=%v, want an in-place update", isNew, updated)
	}
}

func TestDeleteDNSRecordTolerates404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"success":false,"errors":[{"code":81044,"message":"Record does not exist."}],"result":null}`)
	}))
	defer srv.Close()

	c := New("t")
	c.BaseURL = srv.URL
	c.ZoneID = "zone-1"

	if err := c.DeleteDNSRecord(context.Background(), "gone"); err != nil {
		t.Fatalf("DeleteDNSRecord() = %v, want nil for an already-deleted record", err)
	}
}

func TestMissingAccountIDIsAClearError(t *testing.T) {
	c := New("t")
	if _, err := c.ListTunnels(context.Background(), ""); err == nil {
		t.Fatal("expected an error without an account id")
	}
}

func TestZoneNameFor(t *testing.T) {
	zones := []Zone{
		{ID: "1", Name: "example.com"},
		{ID: "2", Name: "co.example.com"},
	}
	got, ok := ZoneNameFor("api.co.example.com", zones)
	if !ok || got.ID != "2" {
		t.Fatalf("ZoneNameFor picked %+v, want the longest match", got)
	}
	if _, ok := ZoneNameFor("other.org", zones); ok {
		t.Fatal("ZoneNameFor matched an unrelated domain")
	}
}
