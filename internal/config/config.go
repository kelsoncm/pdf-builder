package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// User represents an authenticated user.
type User struct {
	Username    string `mapstructure:"username"`
	Token       string `mapstructure:"token"`
	Description string `mapstructure:"description"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// WkhtmltopdfConfig holds wkhtmltopdf settings.
type WkhtmltopdfConfig struct {
	BinaryPath     string            `mapstructure:"binary_path"`
	TimeoutSeconds int               `mapstructure:"timeout_seconds"`
	DefaultOptions map[string]string `mapstructure:"default_options"`
}

// Config is the top-level configuration structure.
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Wkhtmltopdf WkhtmltopdfConfig `mapstructure:"wkhtmltopdf"`
	Users       []User            `mapstructure:"users"`
}

// Load reads configuration from settings.yaml and environment variables.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "120s")
	v.SetDefault("wkhtmltopdf.binary_path", "/usr/local/bin/wkhtmltopdf")
	v.SetDefault("wkhtmltopdf.timeout_seconds", 60)

	// Config file
	v.SetConfigName("settings")
	v.SetConfigType("yaml")
	if configPath != "" {
		v.AddConfigPath(configPath)
	}
	v.AddConfigPath(".")
	v.AddConfigPath("/app")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Load users from environment variables: PDFSERVICE_USER_<USERNAME>=<TOKEN>
	cfg.loadUsersFromEnv()

	return cfg, nil
}

// loadUsersFromEnv reads PDFSERVICE_USER_<USERNAME>=<TOKEN> environment variables
// and adds/overwrites the users list.
func (c *Config) loadUsersFromEnv() {
	const prefix = "PDFSERVICE_USER_"
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, prefix) {
			continue
		}
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		username := strings.ToLower(strings.TrimPrefix(parts[0], prefix))
		token := parts[1]
		if username == "" || token == "" {
			continue
		}
		// Overwrite if exists, otherwise append.
		found := false
		for i, u := range c.Users {
			if strings.EqualFold(u.Username, username) {
				c.Users[i].Token = token
				found = true
				break
			}
		}
		if !found {
			c.Users = append(c.Users, User{
				Username:    username,
				Token:       token,
				Description: "loaded from environment",
			})
		}
	}
}

// TokenMap returns a map of token → username for fast lookup.
func (c *Config) TokenMap() map[string]string {
	m := make(map[string]string, len(c.Users))
	for _, u := range c.Users {
		if u.Token != "" {
			m[u.Token] = u.Username
		}
	}
	return m
}
