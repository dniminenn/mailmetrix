// incomplete and untested implementation of the Roundcube tester

package webmailtester

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dniminenn/mailmetrix/config"
)

type RoundcubeTester struct {
	cfg         config.WebmailServerConfig
	client      *http.Client
	sessionID   string
	authToken   string
	lastRefresh time.Time
}

func (r *RoundcubeTester) GetName() string {
	return r.cfg.Name
}

func NewRoundcubeTester(cfg config.WebmailServerConfig) WebmailTester {
	return &RoundcubeTester{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func init() {
	Register("roundcube", NewRoundcubeTester)
}

func (r *RoundcubeTester) RunSession(ctx context.Context) {
	if err := r.login(); err != nil {
		webmailErrors.WithLabelValues(r.cfg.Name, "login").Inc()
		return
	}

	if err := r.testListing(); err != nil {
		webmailErrors.WithLabelValues(r.cfg.Name, "listing").Inc()
	}

	if err := r.testMessageLoad(); err != nil {
		webmailErrors.WithLabelValues(r.cfg.Name, "loading").Inc()
	}
}

func (r *RoundcubeTester) login() error {
	start := time.Now()

	loginURL := fmt.Sprintf("%s/?_task=login", r.cfg.BaseURL)

	formData := map[string]string{
		"_task":   "login",
		"_action": "login",
		"_user":   r.cfg.Username,
		"_pass":   r.cfg.Password,
	}
	formBody := make([]byte, 0)
	for key, value := range formData {
		formBody = append(formBody, []byte(fmt.Sprintf("%s=%s&", key, value))...)
	}
	formBody = formBody[:len(formBody)-1]

	req, err := http.NewRequest("POST", loginURL, bytes.NewReader(formBody))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(body))
	}

	// incomplete session management
	r.sessionID = resp.Header.Get("Set-Cookie")
	r.authToken = extractAuthToken(resp.Body)

	loginDuration := time.Since(start)
	webmailLoginTime.WithLabelValues(r.cfg.Name).Set(loginDuration.Seconds())
	return nil
}

func (r *RoundcubeTester) testListing() error {
	start := time.Now()

	listURL := fmt.Sprintf("%s/?_task=mail&_action=list", r.cfg.BaseURL)

	req, err := http.NewRequest("GET", listURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create list request: %w", err)
	}

	req.Header.Set("Cookie", r.sessionID)
	req.Header.Set("X-Roundcube-Auth", r.authToken)

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("list request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("listing failed with status %d: %s", resp.StatusCode, string(body))
	}

	listingDuration := time.Since(start)
	webmailFirstPageTime.WithLabelValues(r.cfg.Name).Set(listingDuration.Seconds())
	return nil
}

func (r *RoundcubeTester) testMessageLoad() error {
	start := time.Now()

	loadURL := fmt.Sprintf("%s/?_task=mail&_action=preview&_uid=1", r.cfg.BaseURL)

	req, err := http.NewRequest("GET", loadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create message load request: %w", err)
	}

	req.Header.Set("Cookie", r.sessionID)
	req.Header.Set("X-Roundcube-Auth", r.authToken)

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("message load request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("message load failed with status %d: %s", resp.StatusCode, string(body))
	}

	loadDuration := time.Since(start)
	webmailMessageLoadTime.WithLabelValues(r.cfg.Name).Set(loadDuration.Seconds())
	return nil
}

// Helper function to extract the auth token from the response body
func extractAuthToken(body io.Reader) string {
	// incomplete implementation
	var result map[string]interface{}
	if err := json.NewDecoder(body).Decode(&result); err == nil {
		if token, ok := result["request_token"].(string); ok {
			return token
		}
	}
	return ""
}
