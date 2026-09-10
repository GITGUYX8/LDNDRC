package join

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client talks to the control panel nodes API.
type Client struct {
	BaseURL string // e.g. http://192.168.1.50:8082
	HTTP    *http.Client
}

// NewClient builds a client with sane timeouts.
func NewClient(baseURL string) *Client {
	return &Client{BaseURL: baseURL, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

// Fingerprint is the laptop's hardware profile for registration.
type Fingerprint struct {
	Hostname string
	OS       string
	Arch     string
	CPU      int
	RAMGB    float64
	GPU      string
	JoinKey  string
}

// PollResult is one status poll. Token fields appear exactly once, on the
// first poll after approval; the token lives in memory only.
type PollResult struct {
	Status   string
	Reason   string
	K3sURL   string `json:"k3s_url"`
	K3sToken string `json:"k3s_token"`
	NodeName string `json:"node_name"`
}

// Register posts the fingerprint; idempotent server-side on re-run.
func (c *Client) Register(ctx context.Context, fp Fingerprint) (id string, err error) {
	body, _ := json.Marshal(map[string]any{
		"hostname": fp.Hostname, "os": fp.OS, "arch": fp.Arch,
		"cpu": fp.CPU, "ram_gb": fp.RAMGB, "gpu": fp.GPU, "join_key": fp.JoinKey,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/nodes/register", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("register: status %d", resp.StatusCode)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// Poll fetches the current join status once.
func (c *Client) Poll(ctx context.Context, id string) (PollResult, error) {
	var out PollResult
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/nodes/"+id, nil)
	if err != nil {
		return out, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("poll: status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	return out, nil
}

// WaitApproval polls every 5s until approved/denied/joined or timeout.
func (c *Client) WaitApproval(ctx context.Context, id string, timeout time.Duration) (PollResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		res, err := c.Poll(ctx, id)
		if err == nil && res.Status != "pending" {
			return res, nil
		}
		select {
		case <-ctx.Done():
			return PollResult{}, fmt.Errorf("approval wait timed out")
		case <-t.C:
		}
	}
}

// WaitJoined polls until the watcher marks the node joined (Ready +
// labeled in the cluster) or the timeout elapses.
func (c *Client) WaitJoined(ctx context.Context, id string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		res, err := c.Poll(ctx, id)
		if err == nil && res.Status == "joined" {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("join confirm timed out")
		case <-t.C:
		}
	}
}

// UploadDiagnostics posts a bundle file; the server caps it at 1MB.
func (c *Client) UploadDiagnostics(ctx context.Context, id string, bundle []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.BaseURL+"/api/nodes/"+id+"/diagnostics", bytes.NewReader(bundle))
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("upload: status %d", resp.StatusCode)
	}
	return nil
}
