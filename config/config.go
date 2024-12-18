package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	IMAP    IMAPConfig    `mapstructure:"imap"`
	Webmail WebmailConfig `mapstructure:"webmail"`
	Metrics MetricsConfig `mapstructure:"metrics"`
}

type IMAPConfig struct {
	Servers []ServerConfig `mapstructure:"servers"`
}

type ServerConfig struct {
	Name     string `mapstructure:"name"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type WebmailConfig struct {
	Servers []WebmailServerConfig `mapstructure:"servers"`
}

type WebmailServerConfig struct {
	Name      string `mapstructure:"name"`
	Type      string `mapstructure:"type"`
	UserAgent string `mapstructure:"user_agent"`
	BaseURL   string `mapstructure:"base_url"`
	Username  string `mapstructure:"username"`
	Password  string `mapstructure:"password"`
}

type MetricsConfig struct {
	PrometheusPort int `mapstructure:"prometheus_port"`
	TestInterval   int `mapstructure:"test_interval"`
}

func LoadConfig(path string) (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("metrics.prometheus_port", 9090)
	v.SetDefault("metrics.test_interval", 30)

	// Configure viper
	v.SetConfigFile(path)
	v.SetEnvPrefix("EMAIL_TESTER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config: %w", err)
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

func validateConfig(cfg *Config) error {
	for i, server := range cfg.IMAP.Servers {
		if err := validateServer(server, "IMAP", i); err != nil {
			return err
		}
	}

	for i, server := range cfg.Webmail.Servers {
		if err := validateWebmailServer(server, i); err != nil {
			return err
		}
	}

	return nil
}

func validateServer(server ServerConfig, serverType string, index int) error {
	if server.Name == "" {
		return fmt.Errorf("%s server %d: name cannot be empty", serverType, index)
	}
	if server.Host == "" {
		return fmt.Errorf("%s server %d: host cannot be empty", serverType, index)
	}
	if server.Port <= 0 || server.Port > 65535 {
		return fmt.Errorf("%s server %d: invalid port number: %d", serverType, index, server.Port)
	}
	if server.Username == "" {
		return fmt.Errorf("%s server %d: username cannot be empty", serverType, index)
	}
	if server.Password == "" {
		return fmt.Errorf("%s server %d: password cannot be empty", serverType, index)
	}
	return nil
}

func validateWebmailServer(server WebmailServerConfig, index int) error {
	if server.Name == "" {
		return fmt.Errorf("webmail server %d: name cannot be empty", index)
	}
	if server.Type == "" {
		return fmt.Errorf("webmail server %d: type cannot be empty", index)
	}
	if server.BaseURL == "" {
		return fmt.Errorf("webmail server %d: base_url cannot be empty", index)
	}
	if !strings.HasPrefix(server.BaseURL, "http://") && !strings.HasPrefix(server.BaseURL, "https://") {
		return fmt.Errorf("webmail server %d: base_url must start with http:// or https://", index)
	}
	return nil
}
