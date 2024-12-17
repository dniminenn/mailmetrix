package imaptester

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/dniminenn/mailmetrix/config"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

type Tester struct {
	cfg    config.ServerConfig
	client atomic.Pointer[client.Client]
}

func (t *Tester) GetName() string {
	return t.cfg.Name
}

func NewTester(cfg config.ServerConfig) *Tester {
	return &Tester{cfg: cfg}
}

func (t *Tester) Authenticate() error {
	if t.client.Load() != nil {
		return fmt.Errorf("connection already exists")
	}

	address := fmt.Sprintf("%s:%d", t.cfg.Host, t.cfg.Port)
	dialer := &net.Dialer{Timeout: 10 * time.Second}

	tlsConfig := &tls.Config{
		ServerName:         t.cfg.Host,
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	}

	var conn net.Conn
	var err error

	conn, err = tls.DialWithDialer(dialer, "tcp", address, tlsConfig)
	if err != nil {
		conn, err = dialer.Dial("tcp", address)
		if err != nil {
			return fmt.Errorf("failed to connect to %s: %w", address, err)
		}
	}

	start := time.Now()
	c, err := client.New(conn)
	if err != nil {
		return fmt.Errorf("failed to initialize IMAP client: %w", err)
	}
	timeToBanner.WithLabelValues(t.cfg.Name).Set(time.Since(start).Seconds())

	start = time.Now()
	if err = c.Login(t.cfg.Username, t.cfg.Password); err != nil {
		c.Logout()
		return fmt.Errorf("login failed: %w", err)
	}

	t.client.Store(c)
	timeToAuth.WithLabelValues(t.cfg.Name).Set(time.Since(start).Seconds())
	return nil
}

func (t *Tester) FetchTest(ctx context.Context) error {
	c := t.client.Load()
	if c == nil {
		return fmt.Errorf("no active connection")
	}

	start := time.Now()
	mbox, err := c.Select("INBOX", false)
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
	done := make(chan error, 1)

	go func() {
		done <- c.Fetch(seqSet, []imap.FetchItem{imap.FetchEnvelope}, messages)
	}()

	for msg := range messages {
		_ = msg
	}

	if err := <-done; err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	timeToFetch.WithLabelValues(t.cfg.Name).Set(time.Since(start).Seconds())
	return nil
}

func (t *Tester) AppendTest(ctx context.Context) error {
	c := t.client.Load()
	if c == nil {
		return fmt.Errorf("no active connection")
	}

	start := time.Now()
	testMessage := "From: jr@mailmetrix.example.org\r\n" +
		"To: rj@mailmetrix.example.org\r\n" +
		"Subject: mailmetrix-test\r\n" +
		"\r\n" +
		"This is a test message for IMAP testing purposes.\r\n"

	if err := c.Append("INBOX", nil, time.Now(), strings.NewReader(testMessage)); err != nil {
		return fmt.Errorf("append failed: %w", err)
	}

	timeToAppend.WithLabelValues(t.cfg.Name).Set(time.Since(start).Seconds())
	return t.cleanupTestMessage()
}

func (t *Tester) cleanupTestMessage() error {
	c := t.client.Load()
	if c == nil {
		return fmt.Errorf("no active connection")
	}

	start := time.Now()
	mbox, err := c.Select("INBOX", false)
	if err != nil {
		return fmt.Errorf("cleanup select failed: %w", err)
	}

	if mbox.Messages <= 1 {
		log.Println("[IMAP] Only one or no message present. Skipping cleanup.")
		return nil
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddRange(1, mbox.Messages-1)

	if err := c.Store(seqSet, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.DeletedFlag}, nil); err != nil {
		return fmt.Errorf("failed to mark messages as deleted: %w", err)
	}

	if err := c.Expunge(nil); err != nil {
		return fmt.Errorf("failed to expunge messages: %w", err)
	}

	timeToExpunge.WithLabelValues(t.cfg.Name).Set(time.Since(start).Seconds())
	return nil
}

func (t *Tester) RunSession(ctx context.Context) error {
	errChan := make(chan error, 1)

	go func() {
		if err := t.Authenticate(); err != nil {
			errChan <- fmt.Errorf("authentication failed: %w", err)
			return
		}
		defer func() {
			if c := t.client.Swap(nil); c != nil {
				c.Logout()
			}
		}()
		if err := t.AppendTest(ctx); err != nil {
			errChan <- fmt.Errorf("append test failed: %w", err)
			return
		}

		if err := t.FetchTest(ctx); err != nil {
			errChan <- fmt.Errorf("fetch test failed: %w", err)
			return
		}
		errChan <- nil
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		if c := t.client.Swap(nil); c != nil {
			c.Logout()
		}
		return fmt.Errorf("session timed out: %w", ctx.Err())
	}
}
