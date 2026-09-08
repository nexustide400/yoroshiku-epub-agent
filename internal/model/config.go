package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("設定JSONを読めません: %w", err)
	}
	if cfg.InputDir == "" {
		cfg.InputDir = filepath.Dir(path)
	} else if !filepath.IsAbs(cfg.InputDir) {
		cfg.InputDir = filepath.Join(filepath.Dir(path), cfg.InputDir)
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = filepath.Join(cfg.InputDir, "release")
	} else if !filepath.IsAbs(cfg.OutputDir) {
		cfg.OutputDir = filepath.Join(filepath.Dir(path), cfg.OutputDir)
	}
	cfg.ApplyDefaults()
	return cfg, nil
}

func SaveConfig(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (c *Config) ApplyDefaults() {
	if c.Version == 0 {
		c.Version = 1
	}
	if c.Book.Language == "" {
		c.Book.Language = "ja"
	}
	if c.Book.WritingMode == "" {
		c.Book.WritingMode = "horizontal-tb"
	}
	if c.Book.Modified == "" {
		c.Book.Modified = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	}
	if c.Book.Identifier == "" {
		h := sha256.Sum256([]byte(strings.Join([]string{c.Book.Title, c.Book.Creator, c.Book.Language}, "\x00")))
		c.Book.Identifier = "urn:uuid:" + formatUUID(hex.EncodeToString(h[:16]))
	}
	for i := range c.Sections {
		if c.Sections[i].Level <= 0 {
			c.Sections[i].Level = 1
		}
	}
}

func (c Config) Validate() error {
	var missing []string
	if strings.TrimSpace(c.Book.Title) == "" {
		missing = append(missing, "book.title")
	}
	if strings.TrimSpace(c.Book.Creator) == "" {
		missing = append(missing, "book.creator")
	}
	if strings.TrimSpace(c.Cover.Path) == "" {
		missing = append(missing, "cover.path")
	}
	if len(c.Sections) == 0 {
		missing = append(missing, "sections")
	}
	if len(missing) > 0 {
		return fmt.Errorf("必須設定が不足しています: %s", strings.Join(missing, ", "))
	}
	if c.Version != 1 {
		return fmt.Errorf("未対応の設定バージョンです: %d", c.Version)
	}
	if c.Book.WritingMode != "horizontal-tb" {
		return fmt.Errorf("v1で対応するwritingModeはhorizontal-tbだけです: %s", c.Book.WritingMode)
	}
	return nil
}

func formatUUID(hex32 string) string {
	if len(hex32) < 32 {
		return hex32
	}
	return hex32[:8] + "-" + hex32[8:12] + "-" + hex32[12:16] + "-" + hex32[16:20] + "-" + hex32[20:32]
}
