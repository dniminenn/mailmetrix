package imaptester

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/dniminenn/mailmetrix/config"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

type Tester struct {
	cfg    config.ServerConfig
	client *client.Client
	mu     sync.Mutex
}

func (t *Tester) GetName() string {
	return t.cfg.Name
}

func NewTester(cfg config.ServerConfig) *Tester {
	return &Tester{cfg: cfg}
}

// Authenticate establishes a fresh connection and logs in.
func (t *Tester) Authenticate() error {
	if !t.tryLock(2 * time.Second) {
		return fmt.Errorf("failed to acquire lock for authentication")
	}
	defer t.mu.Unlock()

	t.cleanup()

	address := fmt.Sprintf("%s:%d", t.cfg.Host, t.cfg.Port)

	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}

	tlsConfig := &tls.Config{
		ServerName:         t.cfg.Host,
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	}

	var conn net.Conn
	var err error

	conn, err = tls.DialWithDialer(dialer, "tcp", address, tlsConfig)
	if err != nil {
		// Retry without TLS (plaintext connection)
		conn, err = dialer.Dial("tcp", address)
		if err != nil {
			return fmt.Errorf("failed to connect to %s: %w", address, err)
		}
	}

	t.client, err = client.New(conn)
	if err != nil {
		return fmt.Errorf("failed to initialize IMAP client: %w", err)
	}

	start := time.Now()
	if err = t.client.Login(t.cfg.Username, t.cfg.Password); err != nil {
		t.cleanup()
		return fmt.Errorf("login failed: %w", err)
	}

	timeToAuth.WithLabelValues(t.cfg.Name).Set(time.Since(start).Seconds())

	return nil
}

// FetchTest verifies retrieval.
func (t *Tester) FetchTest(ctx context.Context) error {
	if !t.tryLock(2 * time.Second) {
		return fmt.Errorf("failed to acquire lock for fetch test")
	}
	defer t.mu.Unlock()

	if t.client == nil {
		return fmt.Errorf("no active connection")
	}

	start := time.Now()
	mbox, err := t.client.Select("INBOX", false)
	if err != nil {
		return fmt.Errorf("failed to select INBOX: %w", err)
	}

	if mbox.Messages == 0 {
		log.Println("[IMAP] No messages in INBOX")
		return nil
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddRange(1, mbox.Messages)

	messages := make(chan *imap.Message, 10)
	if err := t.client.Fetch(seqSet, []imap.FetchItem{imap.FetchEnvelope}, messages); err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}
	timeToFetch.WithLabelValues(t.cfg.Name).Set(time.Since(start).Seconds())
	return nil
}

// AppendTest appends a test message.
func (t *Tester) AppendTest(ctx context.Context) error {
	if !t.tryLock(2 * time.Second) {
		return fmt.Errorf("failed to acquire lock for append test")
	}
	defer t.mu.Unlock()

	if t.client == nil {
		return fmt.Errorf("no active connection")
	}

	start := time.Now()
	testMessage := "From: jr@mailmetrix.example.org\r\n" +
		"To: rj@mailmetrix.example.org\r\n" +
		"Subject: mailmetrix-test\r\n" +
		"\r\n" +
		"This is a test message for IMAP testing purposes.\r\n"

	if err := t.client.Append("INBOX", nil, time.Now(), strings.NewReader(testMessage)); err != nil {
		t.cleanup()
		if err = t.Authenticate(); err != nil {
			return fmt.Errorf("re-authentication failed: %w", err)
		}
		if err := t.client.Append("INBOX", nil, time.Now(), strings.NewReader(testMessage)); err != nil {
			return fmt.Errorf("append retry failed: %w", err)
		}
	}
	timeToAppend.WithLabelValues(t.cfg.Name).Set(time.Since(start).Seconds())
	return t.cleanupTestMessage()
}

// cleanupTestMessage removes the appended test message.
func (t *Tester) cleanupTestMessage() error {
	start := time.Now()

	mbox, err := t.client.Select("INBOX", false)
	if err != nil {
		return fmt.Errorf("cleanup select failed: %w", err)
	}

	if mbox.Messages <= 1 {
		log.Println("[IMAP] Only one or no message present. Skipping cleanup.")
		return nil
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddRange(1, mbox.Messages-1)

	if err := t.client.Store(seqSet, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.DeletedFlag}, nil); err != nil {
		return fmt.Errorf("failed to mark messages as deleted: %w", err)
	}

	if err := t.client.Expunge(nil); err != nil {
		return fmt.Errorf("failed to expunge messages: %w", err)
	}

	timeToExpunge.WithLabelValues(t.cfg.Name).Set(time.Since(start).Seconds())
	return nil
}

// RunSession is the entry point for the test session.
func (t *Tester) RunSession(ctx context.Context) {
	if err := t.Authenticate(); err != nil {
		log.Printf("[IMAP] Authentication failed: %v", err)
		return
	}
	defer t.cleanup()

	if err := t.AppendTest(ctx); err != nil {
		log.Printf("[IMAP] Append test failed: %v", err)
	}

	if err := t.FetchTest(ctx); err != nil {
		log.Printf("[IMAP] Fetch test failed: %v", err)
	}
}

// cleanup ensures any connection is closed cleanly.
func (t *Tester) cleanup() {
	if t.client != nil {
		_ = t.client.Logout()
		t.client = nil
	}
}

// tryLock attempts to acquire the mutex with a timeout.
func (t *Tester) tryLock(timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	lockChan := make(chan struct{})
	go func() {
		t.mu.Lock()
		close(lockChan)
	}()

	select {
	case <-lockChan:
		return true
	case <-timer.C:
		return false
	}
}
