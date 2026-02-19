package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	content := []byte(`
app:
  name: "Test App"
  port: 9090
  debug: true
  session_timeout: 900
  session_secret: "test-secret"
  session_name: "_test_app"
  password_salt: "test-salt"
database:
  host: localhost
  port: 3306
  name: testdb
  user: testuser
  password: testpass
  charset: utf8mb4
redis:
  addr: localhost:6379
tms:
  base_url: "http://localhost:8280"
`)
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	_, err = tmpFile.Write(content)
	require.NoError(t, err)
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	require.NoError(t, err)
	assert.Equal(t, "Test App", cfg.App.Name)
	assert.Equal(t, 9090, cfg.App.Port)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, "testdb", cfg.Database.Name)
	assert.Equal(t, "localhost:6379", cfg.Redis.Addr)
	assert.Equal(t, "http://localhost:8280", cfg.TMS.BaseURL)
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	assert.Error(t, err)
}

func TestConfig_DSN(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     3306,
			Name:     "testdb",
			User:     "root",
			Password: "pass",
			Charset:  "utf8mb4",
		},
	}
	expected := "root:pass@tcp(localhost:3306)/testdb?charset=utf8mb4&parseTime=True&loc=Local"
	assert.Equal(t, expected, cfg.Database.DSN())
}
