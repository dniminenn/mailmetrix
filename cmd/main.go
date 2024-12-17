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

func runWebmailTests(ctx context.Context, webmailTesters []webmailtester.WebmailTester,
	lock chan struct{}, testInterval time.Duration) {
	select {
	case lock <- struct{}{}:
		defer func() { <-lock }()

		ctx, cancel := context.WithTimeout(ctx, testInterval*3)
		defer cancel()

		var wg sync.WaitGroup
		for _, tester := range webmailTesters {
			wg.Add(1)
			go func(t webmailtester.WebmailTester) {
				defer wg.Done()
				if err := t.RunSession(ctx); err != nil {
					log.Printf("Webmail test for server %s failed: %v", t.GetName(), err)
				}
			}(tester)
		}

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			return
		case <-ctx.Done():
			log.Printf("Webmail tests timed out")
			return
		}
	default:
		log.Println("Webmail tests are still running, skipping this iteration.")
	}
}

func runIMAPTests(ctx context.Context, imapTesters []*imaptester.Tester,
	lock chan struct{}, testInterval time.Duration) {
	select {
	case lock <- struct{}{}:
		defer func() { <-lock }()

		ctx, cancel := context.WithTimeout(ctx, testInterval*3)
		defer cancel()

		var wg sync.WaitGroup
		for _, tester := range imapTesters {
			wg.Add(1)
			go func(t *imaptester.Tester) {
				defer wg.Done()
				if err := t.RunSession(ctx); err != nil {
					log.Printf("IMAP test for server %s failed: %v", t.GetName(), err)
				}
			}(tester)
		}

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			return
		case <-ctx.Done():
			log.Printf("IMAP tests timed out")
			return
		}
	default:
		log.Println("IMAP tests are still running, skipping this iteration.")
	}
}
