package webmailtester

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"strings"
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

func (r *RoundcubeTester) RunSession(ctx context.Context) error {
	errChan := make(chan error, 1)

	go func() {
		if err := r.login(); err != nil {
			errChan <- fmt.Errorf("login failed: %w", err)
			webmailErrors.WithLabelValues(r.cfg.Name, "login").Inc()
			return
		}

		defer func() {
			r.sessionID = ""
			r.authToken = ""
		}()

		if err := r.testListing(); err != nil {
			errChan <- fmt.Errorf("listing test failed: %w", err)
			webmailErrors.WithLabelValues(r.cfg.Name, "listing").Inc()
			return
		}

		if err := r.testMessageLoad(); err != nil {
			errChan <- fmt.Errorf("message load test failed: %w", err)
			webmailErrors.WithLabelValues(r.cfg.Name, "loading").Inc()
			return
		}

		errChan <- nil
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		r.sessionID = ""
		r.authToken = ""
		return fmt.Errorf("roundcube session timed out: %w", ctx.Err())
	}
}

func (r *RoundcubeTester) login() error {
	start := time.Now()

	loginURL := fmt.Sprintf("%s/?_task=login", r.cfg.BaseURL)
	formData := fmt.Sprintf("_task=login&_action=login&_user=%s&_pass=%s", r.cfg.Username, r.cfg.Password)

	// TTFB Tracking
	var ttfb time.Duration
	trace := &httptrace.ClientTrace{
		GotFirstResponseByte: func() {
			ttfb = time.Since(start)
			webmailTTFB.WithLabelValues(r.cfg.Name).Set(ttfb.Seconds())
		},
	}

	req, err := http.NewRequest("POST", loginURL, strings.NewReader(formData))
	if err != nil {
		handleFailure(r.cfg.Name, "login", err)
		return fmt.Errorf("failed to create login request: %w", err)
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := r.client.Do(req)
	if err != nil {
		handleFailure(r.cfg.Name, "login", err)
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		handleFailure(r.cfg.Name, "login", fmt.Errorf("status code: %d, body: %s", resp.StatusCode, string(body)))
		return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(body))
	}

	r.sessionID = extractSessionID(resp.Header)
	r.authToken = extractAuthToken(resp.Body)
	if r.sessionID == "" || r.authToken == "" {
		err := fmt.Errorf("failed to retrieve session ID or auth token")
		handleFailure(r.cfg.Name, "login", err)
		return err
	}

	loginDuration := time.Since(start)
	webmailLoginTime.WithLabelValues(r.cfg.Name).Set(loginDuration.Seconds())
	return nil
}

func (r *RoundcubeTester) testListing() error {
	start := time.Now()
	listURL := fmt.Sprintf("%s/?_task=mail&_action=list", r.cfg.BaseURL)

	req, err := http.NewRequest("GET", listURL, nil)
	if err != nil {
		handleFailure(r.cfg.Name, "listing", err)
		return fmt.Errorf("failed to create list request: %w", err)
	}

	req.Header.Set("Cookie", r.sessionID)
	req.Header.Set("X-Roundcube-Auth", r.authToken)

	var ttfb time.Duration
	trace := &httptrace.ClientTrace{
		GotFirstResponseByte: func() {
			ttfb = time.Since(start)
			webmailTTFB.WithLabelValues(r.cfg.Name).Set(ttfb.Seconds())
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	resp, err := r.client.Do(req)
	if err != nil {
		handleFailure(r.cfg.Name, "listing", err)
		return fmt.Errorf("list request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		handleFailure(r.cfg.Name, "listing", fmt.Errorf("status code: %d, body: %s", resp.StatusCode, string(body)))
		return fmt.Errorf("listing failed with status %d: %s", resp.StatusCode, string(body))
	}

	listDuration := time.Since(start)
	webmailFirstPageTime.WithLabelValues(r.cfg.Name).Set(listDuration.Seconds())
	return nil
}

func (r *RoundcubeTester) testMessageLoad() error {
	start := time.Now()
	loadURL := fmt.Sprintf("%s/?_task=mail&_action=preview&_uid=1", r.cfg.BaseURL)

	req, err := http.NewRequest("GET", loadURL, nil)
	if err != nil {
		handleFailure(r.cfg.Name, "loading", err)
		return fmt.Errorf("failed to create message load request: %w", err)
	}

	req.Header.Set("Cookie", r.sessionID)
	req.Header.Set("X-Roundcube-Auth", r.authToken)

	var ttfb time.Duration
	trace := &httptrace.ClientTrace{
		GotFirstResponseByte: func() {
			ttfb = time.Since(start)
			webmailTTFB.WithLabelValues(r.cfg.Name).Set(ttfb.Seconds())
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	resp, err := r.client.Do(req)
	if err != nil {
		handleFailure(r.cfg.Name, "loading", err)
		return fmt.Errorf("message load request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		handleFailure(r.cfg.Name, "loading", fmt.Errorf("status code: %d, body: %s", resp.StatusCode, string(body)))
		return fmt.Errorf("message load failed with status %d: %s", resp.StatusCode, string(body))
	}

	loadDuration := time.Since(start)
	webmailMessageLoadTime.WithLabelValues(r.cfg.Name).Set(loadDuration.Seconds())
	return nil
}

func extractSessionID(header http.Header) string {
	cookies := header["Set-Cookie"]
	for _, cookie := range cookies {
		if strings.Contains(cookie, "roundcube_sessid") {
			return strings.Split(cookie, ";")[0]
		}
	}
	return ""
}

func extractAuthToken(body io.Reader) string {
	var result map[string]interface{}
	if err := json.NewDecoder(body).Decode(&result); err == nil {
		if token, ok := result["request_token"].(string); ok {
			return token
		}
	}
	return ""
}
