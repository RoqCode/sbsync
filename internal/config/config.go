package config

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Token       string
	SourceSpace string
	TargetSpace string
	Path        string
}

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".sbrc" // fallback to current directory
	}
	return filepath.Join(home, ".sbrc")
}

func Load(path string) (Config, error) {
	cfg := Config{Path: path}
	if env := os.Getenv("SB_TOKEN"); env != "" {
		cfg.Token = strings.TrimSpace(env)
	}
	f, err := os.Open(path)
	if err != nil {
		if cfg.Token != "" {
			return cfg, nil
		}
		return cfg, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		switch k {
		case "SB_TOKEN":
			if cfg.Token == "" {
				cfg.Token = v
			}
		case "SOURCE_SPACE_ID":
			cfg.SourceSpace = v
		case "TARGET_SPACE_ID":
			cfg.TargetSpace = v
		}
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if strings.TrimSpace(cfg.Token) == "" {
		return errors.New("kein Token zum Speichern")
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	content := []string{
		"SB_TOKEN=" + cfg.Token,
	}
	if cfg.SourceSpace != "" {
		content = append(content, "SOURCE_SPACE_ID="+cfg.SourceSpace)
	}
	if cfg.TargetSpace != "" {
		content = append(content, "TARGET_SPACE_ID="+cfg.TargetSpace)
	}
	return os.WriteFile(path, []byte(strings.Join(content, "\n")+"\n"), 0o600)
}
