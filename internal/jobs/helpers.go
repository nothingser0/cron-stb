package jobs

import (
	"bufio"
	"os"
	"path/filepath"
	"time"

	"cron-stb/internal/config"
)

func getWIBLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	return loc
}

func getWIBNow() time.Time {
	return time.Now().In(getWIBLocation())
}

func getHermesWorkspaceDirs() []string {
	var dirs []string
	if val := config.GetString("HERMES_WORKSPACE_DIR", ""); val != "" {
		dirs = append(dirs, val)
	}
	if val := config.GetString("WORKSPACE_DIR", ""); val != "" {
		dirs = append(dirs, val)
	}
	// Default fallback path di STB
	dirs = append(dirs, "/media/devmon/docker-disk/hermes-data/workspace")
	return dirs
}

func findDataFile(category, filename string) string {
	cfg := config.LoadConfig()
	var candidates []string

	for _, ws := range getHermesWorkspaceDirs() {
		candidates = append(candidates,
			filepath.Join(ws, "data", category, filename),
			filepath.Join(ws, category, filename),
		)
	}

	candidates = append(candidates,
		filepath.Join(cfg.BaseDataDir, category, filename),
		filepath.Join(cfg.BaseDataDir, "workspace", "data", category, filename),
		filepath.Join("data", category, filename),
		filepath.Join("workspace", "data", category, filename),
		filepath.Join("/opt/data", category, filename),
		filepath.Join("/opt/data/workspace/data", category, filename),
		filepath.Join("/app/data", category, filename),
	)

	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

func findDataGlob(category, pattern string) []string {
	cfg := config.LoadConfig()
	var baseDirs []string

	for _, ws := range getHermesWorkspaceDirs() {
		baseDirs = append(baseDirs,
			filepath.Join(ws, "data", category),
			filepath.Join(ws, category),
		)
	}

	baseDirs = append(baseDirs,
		filepath.Join(cfg.BaseDataDir, category),
		filepath.Join(cfg.BaseDataDir, "workspace", "data", category),
		filepath.Join("data", category),
		filepath.Join("workspace", "data", category),
		filepath.Join("/opt/data", category),
		filepath.Join("/opt/data/workspace/data", category),
		filepath.Join("/app/data", category),
	)

	seen := make(map[string]bool)
	var matches []string

	for _, dir := range baseDirs {
		p := filepath.Join(dir, pattern)
		files, err := filepath.Glob(p)
		if err == nil {
			for _, f := range files {
				if !seen[f] {
					seen[f] = true
					matches = append(matches, f)
				}
			}
		}
	}
	return matches
}

func readLines(filePath string) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		text := scanner.Text()
		if text != "" {
			lines = append(lines, text)
		}
	}
	return lines, scanner.Err()
}
