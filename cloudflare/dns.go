package cloudflare

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// DNSRecord is a record in a zone.
type DNSRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	TTL     int    `json:"ttl"`
	Comment string `json:"comment,omitempty"`
}

// TunnelCNAMETarget is the CNAME value that routes a hostname into a tunnel.
func TunnelCNAMETarget(tunnelID string) string {
	return tunnelID + ".cfargotunnel.com"
}

// IsTunnelTarget reports whether a CNAME content points at a cloudflared tunnel.
func IsTunnelTarget(content string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSuffix(content, ".")), ".cfargotunnel.com")
}

func (c *Client) zonePath(suffix string) (string, error) {
	if strings.TrimSpace(c.ZoneID) == "" {
		return "", fmt.Errorf("no cloudflare zone id configured")
	}
	return "/zones/" + c.ZoneID + suffix, nil
}

// FindDNSRecord returns the record with the given name, or (nil, nil).
func (c *Client) FindDNSRecord(ctx context.Context, name string) (*DNSRecord, error) {
	path, err := c.zonePath("/dns_records?per_page=50&name=" + url.QueryEscape(strings.ToLower(name)))
	if err != nil {
		return nil, err
	}
	var records []DNSRecord
	if err := c.do(ctx, http.MethodGet, path, nil, &records); err != nil {
		return nil, err
	}
	for i := range records {
		if strings.EqualFold(records[i].Name, name) {
			return &records[i], nil
		}
	}
	return nil, nil
}

// ListDNSRecords returns all records in the zone.
func (c *Client) ListDNSRecords(ctx context.Context) ([]DNSRecord, error) {
	path, err := c.zonePath("/dns_records?per_page=100")
	if err != nil {
		return nil, err
	}
	var records []DNSRecord
	if err := c.do(ctx, http.MethodGet, path, nil, &records); err != nil {
		return nil, err
	}
	return records, nil
}

type dnsRecordPayload struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	TTL     int    `json:"ttl"`
	Comment string `json:"comment,omitempty"`
}

// CreateCNAME adds a proxied CNAME record.
func (c *Client) CreateCNAME(ctx context.Context, name, content string) (*DNSRecord, error) {
	path, err := c.zonePath("/dns_records")
	if err != nil {
		return nil, err
	}
	payload := dnsRecordPayload{
		Type:    "CNAME",
		Name:    strings.ToLower(name),
		Content: content,
		Proxied: true,
		TTL:     1, // automatic
		Comment: "managed by linko",
	}
	var rec DNSRecord
	if err := c.do(ctx, http.MethodPost, path, payload, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// UpdateCNAME replaces an existing record.
func (c *Client) UpdateCNAME(ctx context.Context, id, name, content string) (*DNSRecord, error) {
	path, err := c.zonePath("/dns_records/" + url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	payload := dnsRecordPayload{
		Type:    "CNAME",
		Name:    strings.ToLower(name),
		Content: content,
		Proxied: true,
		TTL:     1,
		Comment: "managed by linko",
	}
	var rec DNSRecord
	if err := c.do(ctx, http.MethodPut, path, payload, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// DeleteDNSRecord removes a record by id.
func (c *Client) DeleteDNSRecord(ctx context.Context, id string) error {
	path, err := c.zonePath("/dns_records/" + url.PathEscape(id))
	if err != nil {
		return err
	}
	err = c.do(ctx, http.MethodDelete, path, nil, nil)
	if apiErr, ok := err.(*Error); ok && apiErr.IsNotFound() {
		return nil
	}
	return err
}

// EnsureCNAME makes sure name is a proxied CNAME pointing at target.
// created reports whether a new record was added.
func (c *Client) EnsureCNAME(ctx context.Context, name, target string) (rec *DNSRecord, created bool, err error) {
	existing, err := c.FindDNSRecord(ctx, name)
	if err != nil {
		return nil, false, err
	}
	if existing == nil {
		rec, err = c.CreateCNAME(ctx, name, target)
		return rec, true, err
	}
	if !strings.EqualFold(existing.Type, "CNAME") {
		return nil, false, fmt.Errorf("%s already exists as a %s record — remove it in the Cloudflare dashboard first", name, existing.Type)
	}
	if strings.EqualFold(strings.TrimSuffix(existing.Content, "."), strings.TrimSuffix(target, ".")) && existing.Proxied {
		return existing, false, nil
	}
	if !IsTunnelTarget(existing.Content) {
		return nil, false, fmt.Errorf("%s already points at %s which is not a cloudflare tunnel — remove it first", name, existing.Content)
	}
	rec, err = c.UpdateCNAME(ctx, existing.ID, name, target)
	return rec, false, err
}
