package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/dniminenn/mailmetrix/config"
	"github.com/dniminenn/mailmetrix/imaptester"
	"github.com/dniminenn/mailmetrix/webmailtester"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	configPath := "/etc/mailmetrix/config.yaml"
	if info, err := os.Stat("./config.yaml"); err == nil && !info.IsDir() {
		configPath = "./config.yaml"
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var imapTesters []*imaptester.Tester
	for _, server := range cfg.IMAP.Servers {
		imapTesters = append(imapTesters, imaptester.NewTester(server))
	}

	var webmailTesters []webmailtester.WebmailTester
	for _, server := range cfg.Webmail.Servers {
		wtester, err := webmailtester.NewWebmailTester(server)
		if err != nil {
			log.Printf("Skipping webmail server %s: %v", server.Name, err)
			continue
		}
		webmailTesters = append(webmailTesters, wtester)
	}

	imapLock := make(chan struct{}, 1)
	webmailLock := make(chan struct{}, 1)

	ticker := time.NewTicker(time.Duration(cfg.Metrics.TestInterval) * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				go runIMAPTests(ctx, imapTesters, imapLock, time.Duration(cfg.Metrics.TestInterval)*time.Second*2)
				go runWebmailTests(ctx, webmailTesters, webmailLock, time.Duration(cfg.Metrics.TestInterval)*time.Second*2)
			case <-ctx.Done():
				return
			}
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	address := fmt.Sprintf(":%d", cfg.Metrics.PrometheusPort)
	log.Printf("Serving metrics on %s", address)
	if err := http.ListenAndServe(address, nil); err != nil {
		log.Fatalf("Failed to start metrics server: %v", err)
	}
}

func runIMAPTests(ctx context.Context, imapTesters []*imaptester.Tester,
	lock chan struct{}, testInterval time.Duration) {
	select {
	case lock <- struct{}{}:
		defer func() { <-lock }()

		timeout := testInterval * 3

		testCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		var wg sync.WaitGroup
		for _, tester := range imapTesters {
			wg.Add(1)
			go func(t *imaptester.Tester) {
				defer wg.Done()
				// Run test with timeout-aware context
				done := make(chan struct{})
				go func() {
					t.RunSession(testCtx)
					close(done)
				}()

				select {
				case <-done:
					// Test completed successfully
					return
				case <-testCtx.Done():
					// Timeout or cancellation occurred
					log.Printf("IMAP test for server %s timed out", t.GetName())
				}
			}(tester)
		}

		wg.Wait()
	default:
		log.Println("IMAP tests are still running, skipping this iteration.")
	}
}

func runWebmailTests(ctx context.Context, webmailTesters []webmailtester.WebmailTester,
	lock chan struct{}, testInterval time.Duration) {
	select {
	case lock <- struct{}{}:
		defer func() { <-lock }()

		timeout := testInterval * 3

		testCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		var wg sync.WaitGroup
		for _, tester := range webmailTesters {
			wg.Add(1)
			go func(t webmailtester.WebmailTester) {
				defer wg.Done()
				// Run test with timeout-aware context
				done := make(chan struct{})
				go func() {
					t.RunSession(testCtx)
					close(done)
				}()

				select {
				case <-done:
					// Test completed successfully
					return
				case <-testCtx.Done():
					// Timeout or cancellation occurred
					log.Printf("Webmail test for server %s timed out", t.GetName())
				}
			}(tester)
		}

		wg.Wait()
	default:
		log.Println("Webmail tests are still running, skipping this iteration.")
	}
}
