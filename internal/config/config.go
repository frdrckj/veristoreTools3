package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App         AppConfig      `yaml:"app"`
	Database    DatabaseConfig `yaml:"database"`
	V2Database  DatabaseConfig `yaml:"v2_database"`
	Redis       RedisConfig    `yaml:"redis"`
	TMS         TMSConfig      `yaml:"tms"`
	API         APIConfig      `yaml:"api"`
	Import      ImportConfig   `yaml:"import"`
	Export      ExportConfig   `yaml:"export"`
	CountryID   int            `yaml:"country_id"`
	PackageName string         `yaml:"package_name"`
}

type AppConfig struct {
	Name           string `yaml:"name"`
	Version        string `yaml:"version"`
	Port           int    `yaml:"port"`
	Debug          bool   `yaml:"debug"`
	SessionTimeout int    `yaml:"session_timeout"`
	SessionSecret  string `yaml:"session_secret"`
	SessionName    string `yaml:"session_name"`
	PasswordSalt   string `yaml:"password_salt"`
}

type DatabaseConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Name         string `yaml:"name"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	Charset      string `yaml:"charset"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		d.User, d.Password, d.Host, d.Port, d.Name, d.Charset)
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type TMSConfig struct {
	BaseURL       string  `yaml:"base_url"`
	APIBaseURL    string  `yaml:"api_base_url"`
	SecretKey     string  `yaml:"secret_key"`
	SecretIV      string  `yaml:"secret_iv"`
	AccessKey     string  `yaml:"access_key"`
	AccessSecret  string  `yaml:"access_secret"`
	SkipTLSVerify bool    `yaml:"skip_tls_verify"`
	SyncBatchSize int     `yaml:"sync_batch_size"`
	ResellerList  []int64 `yaml:"reseller_list"`
}

type APIConfig struct {
	BasicAuthUser string `yaml:"basic_auth_user"`
	BasicAuthPass string `yaml:"basic_auth_pass"`
}

type ImportConfig struct {
	MaxFileSize int `yaml:"max_file_size"`
	BatchSize   int `yaml:"batch_size"`
}

type ExportConfig struct {
	BatchSize int    `yaml:"batch_size"`
	OutputDir string `yaml:"output_dir"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Defaults
	if cfg.App.Port == 0 {
		cfg.App.Port = 8080
	}
	if cfg.Database.Charset == "" {
		cfg.Database.Charset = "utf8mb4"
	}
	if cfg.Database.MaxOpenConns == 0 {
		cfg.Database.MaxOpenConns = 25
	}
	if cfg.Database.MaxIdleConns == 0 {
		cfg.Database.MaxIdleConns = 10
	}
	if cfg.V2Database.Charset == "" {
		cfg.V2Database.Charset = "utf8mb4"
	}
	if cfg.V2Database.MaxOpenConns == 0 {
		cfg.V2Database.MaxOpenConns = 10
	}
	if cfg.V2Database.MaxIdleConns == 0 {
		cfg.V2Database.MaxIdleConns = 5
	}
	if cfg.Import.BatchSize == 0 {
		cfg.Import.BatchSize = 100
	}
	if cfg.Export.BatchSize == 0 {
		cfg.Export.BatchSize = 100
	}
	if cfg.TMS.SyncBatchSize == 0 {
		cfg.TMS.SyncBatchSize = 500
	}

	return &cfg, nil
}
