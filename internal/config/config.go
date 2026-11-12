package config

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type Config struct {
	DiscordBotToken string
	DefaultSheetID  string
	CredentialsFile string
	TokenFile       string
	TokenWriteFile  string
	BaseDataDir     string
}

var (
	OverrideDataDir string
	OverrideEnvFile string
)

func LoadConfig() *Config {
	cfg := &Config{}

	if OverrideDataDir != "" {
		cfg.BaseDataDir = OverrideDataDir
	} else if envDir := os.Getenv("DATA_DIR"); envDir != "" {
		cfg.BaseDataDir = envDir
	} else if dirExists("data") {
		cfg.BaseDataDir = "data"
	} else if runtime.GOOS == "windows" {
		cfg.BaseDataDir = "."
	} else {
		if fileExists(".env") || fileExists("token.json") || dirExists("scripts") {
			cfg.BaseDataDir = "."
		} else {
			cfg.BaseDataDir = "/opt/data"
		}
	}

	envPath := OverrideEnvFile
	if envPath == "" {
		envPath = findFile(
			".env",
			"data/.env",
			filepath.Join(cfg.BaseDataDir, ".env"),
			"/opt/data/.env",
		)
	}
	if envPath != "" {
		loadEnvFile(envPath)
	}

	cfg.DiscordBotToken = os.Getenv("DISCORD_BOT_TOKEN")
	cfg.DefaultSheetID = GetString("SHEET_ID", "")

	cfg.CredentialsFile = findFile(
		os.Getenv("CREDENTIALS_FILE"),
		"data/credentials.json",
		"credentials.json",
		filepath.Join(cfg.BaseDataDir, "credentials.json"),
		filepath.Join(cfg.BaseDataDir, "scripts", "credentials.json"),
		"scripts/credentials.json",
		"/opt/data/scripts/credentials.json",
		"/opt/data/credentials.json",
	)

	cfg.TokenFile = findFile(
		os.Getenv("TOKEN_FILE"),
		"data/token.json",
		"token.json",
		filepath.Join(cfg.BaseDataDir, "token.json"),
		filepath.Join(cfg.BaseDataDir, "scripts", "token.json"),
		"scripts/token.json",
		"/opt/data/scripts/token.json",
		"/opt/data/token.json",
	)

	cfg.TokenWriteFile = findFile(
		os.Getenv("TOKEN_WRITE_FILE"),
		"data/token_write.json",
		"token_write.json",
		filepath.Join(cfg.BaseDataDir, "token_write.json"),
		filepath.Join(cfg.BaseDataDir, "scripts", "token_write.json"),
		"scripts/token_write.json",
		"/opt/data/scripts/token_write.json",
		"/opt/data/token_write.json",
	)
	if cfg.TokenWriteFile == "" {
		cfg.TokenWriteFile = cfg.TokenFile
	}

	return cfg
}

func GetString(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func GetInt64(key string, fallback int64) int64 {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func (c *Config) GetJobDiscordToken(jobKey string) string {
	if val := strings.TrimSpace(os.Getenv(jobKey)); val != "" {
		return val
	}
	return c.DiscordBotToken
}

func (c *Config) GetJobSheetID(jobKey string) string {
	if val := strings.TrimSpace(os.Getenv(jobKey)); val != "" {
		return val
	}
	return c.DefaultSheetID
}

func (c *Config) PrintStatus() {
	fmt.Println("Configuration:")
	fmt.Printf("  Data Dir   : %s\n", c.BaseDataDir)
	if c.DiscordBotToken != "" {
		fmt.Println("  Discord    : configured")
	} else {
		fmt.Println("  Discord    : not set (DISCORD_BOT_TOKEN)")
	}
	fmt.Printf("  Sheet ID   : %s\n", c.DefaultSheetID)
	fmt.Printf("  Token File : %s\n", c.TokenFile)
	fmt.Printf("  Creds File : %s\n", c.CredentialsFile)
	if c.TokenWriteFile != "" && c.TokenWriteFile != c.TokenFile {
		fmt.Printf("  Write Token: %s\n", c.TokenWriteFile)
	}
	fmt.Println()
}

func findFile(candidates ...string) string {
	for _, p := range candidates {
		if p != "" && fileExists(p) {
			return p
		}
	}
	return ""
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
	}
	log.Printf("[config] loaded %s", path)
}
