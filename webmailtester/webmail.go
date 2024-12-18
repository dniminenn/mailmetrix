package webmailtester

import (
	"context"
	"fmt"
	"log"
	"math"

	"github.com/dniminenn/mailmetrix/config"
)

type WebmailTester interface {
	RunSession(context.Context) error
	GetName() string
}

var testers = make(map[string]func(cfg config.WebmailServerConfig) WebmailTester)

func Register(name string, factory func(cfg config.WebmailServerConfig) WebmailTester) {
	testers[name] = factory
}

func NewWebmailTester(cfg config.WebmailServerConfig) (WebmailTester, error) {
	factory, ok := testers[cfg.Type]
	if !ok {
		return nil, fmt.Errorf("no webmail tester found for type: %s", cfg.Type)
	}
	return factory(cfg), nil
}

func handleFailure(server, operation string, err error) {
	log.Printf("[ERROR] %s failed for %s: %v", operation, server, err)
	webmailFailures.WithLabelValues(server, operation).Inc()
	resetMetricsForOperation(server, operation)
}

func resetMetricsForOperation(server, operation string) {
	switch operation {
	case "ttfb":
		webmailTTFB.WithLabelValues(server).Set(math.NaN())
	case "login":
		webmailLoginTime.WithLabelValues(server).Set(math.NaN())
	case "listing":
		webmailFirstPageTime.WithLabelValues(server).Set(math.NaN())
	case "loading":
		webmailMessageLoadTime.WithLabelValues(server).Set(math.NaN())
	}
}
