package webmailtester

import (
	"context"
	"fmt"

	"github.com/dniminenn/mailmetrix/config"
)

type WebmailTester interface {
	GetName() string
	RunSession(ctx context.Context)
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
