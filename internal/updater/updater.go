package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	repo         = "davidleitw/xreview"
	binaryName   = "xreview"
	cacheMaxAge  = 24 * time.Hour
	apiURL       = "https://api.github.com/repos/" + repo + "/releases/latest"
)

// githubRelease holds the minimal fields we need from the GitHub API.
type githubRelease struct {
	TagName string `json:"tag_name"`
}

// versionCache stores the last check result on disk.
type versionCache struct {
	LatestVersion string `json:"latest_version"`
	CheckedAt     int64  `json:"checked_at"`
}

// CheckResult is returned by CheckLatestVersion.
type CheckResult struct {
	CurrentVersion string
	LatestVersion  string
	UpdateAvailable bool
}

// cachePath returns the path to the version cache file.
func cachePath() string {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".cache")
	}
	return filepath.Join(dir, "xreview", "latest-version.json")
}

// CheckLatestVersion checks if a newer version is available.
// Uses a local cache to avoid hitting GitHub API on every call (24h TTL).
func CheckLatestVersion(currentVersion string) CheckResult {
	result := CheckResult{CurrentVersion: currentVersion}

	// Try cache first
	if cached, ok := readCache(); ok {
		result.LatestVersion = cached.LatestVersion
		result.UpdateAvailable = isNewer(cached.LatestVersion, currentVersion)
		return result
	}

	// Fetch from GitHub
	latest, err := fetchLatestVersion()
	if err != nil {
		// Network error — just skip, don't block the user
		return result
	}

	result.LatestVersion = latest
	result.UpdateAvailable = isNewer(latest, currentVersion)

	// Write cache (best-effort)
	writeCache(versionCache{
		LatestVersion: latest,
		CheckedAt:     time.Now().Unix(),
	})

	return result
}

// SelfUpdate downloads the latest release binary and replaces the current one.
func SelfUpdate() (newVersion string, err error) {
	// 1. Get latest version tag
	latest, err := fetchLatestVersion()
	if err != nil {
		return "", fmt.Errorf("failed to check latest version: %w", err)
	}

	// 2. Build download URL
	osName := runtime.GOOS
	archName := runtime.GOARCH
	assetName := fmt.Sprintf("%s-%s-%s", binaryName, osName, archName)
	downloadURL := fmt.Sprintf("https://github.com/%s/releases/latest/download/%s", repo, assetName)

	// 3. Find current binary path
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot determine executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("cannot resolve executable path: %w", err)
	}

	// 4. Download to temp file in same directory (for atomic rename)
	dir := filepath.Dir(execPath)
	tmpFile, err := os.CreateTemp(dir, "xreview-update-*")
	if err != nil {
		// If we can't write to the binary's dir, try ~/.local/bin
		installDir := filepath.Join(os.Getenv("HOME"), ".local", "bin")
		os.MkdirAll(installDir, 0o755)
		tmpFile, err = os.CreateTemp(installDir, "xreview-update-*")
		if err != nil {
			return "", fmt.Errorf("cannot create temp file: %w", err)
		}
		execPath = filepath.Join(installDir, binaryName)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // clean up on failure

	resp, err := http.Get(downloadURL)
	if err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return "", fmt.Errorf("download failed: HTTP %d from %s", resp.StatusCode, downloadURL)
	}

	_, err = io.Copy(tmpFile, resp.Body)
	tmpFile.Close()
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}

	// 5. Make executable
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return "", fmt.Errorf("chmod failed: %w", err)
	}

	// 6. Atomic replace
	if err := os.Rename(tmpPath, execPath); err != nil {
		return "", fmt.Errorf("failed to replace binary: %w", err)
	}

	// 7. Invalidate cache
	writeCache(versionCache{
		LatestVersion: latest,
		CheckedAt:     time.Now().Unix(),
	})

	return latest, nil
}

// fetchLatestVersion queries GitHub API for the latest release tag.
func fetchLatestVersion() (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	return normalizeVersion(release.TagName), nil
}

// readCache reads the version cache if it exists and is fresh.
func readCache() (versionCache, bool) {
	data, err := os.ReadFile(cachePath())
	if err != nil {
		return versionCache{}, false
	}

	var cache versionCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return versionCache{}, false
	}

	age := time.Since(time.Unix(cache.CheckedAt, 0))
	if age > cacheMaxAge {
		return versionCache{}, false
	}

	return cache, true
}

// writeCache writes the version cache to disk.
func writeCache(cache versionCache) {
	path := cachePath()
	os.MkdirAll(filepath.Dir(path), 0o755)
	data, _ := json.Marshal(cache)
	os.WriteFile(path, data, 0o644)
}

// normalizeVersion strips the leading "v" from a version string.
func normalizeVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}

// isNewer returns true if latest is newer than current.
// Simple string comparison after normalization — works for semver.
func isNewer(latest, current string) bool {
	latest = normalizeVersion(latest)
	current = normalizeVersion(current)

	if current == "dev" || current == "" {
		return false // dev builds don't trigger updates
	}

	return latest != current && compareSemver(latest, current) > 0
}

// compareSemver compares two semver strings (without "v" prefix).
// Returns >0 if a > b, 0 if equal, <0 if a < b.
func compareSemver(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	for i := 0; i < 3; i++ {
		var na, nb int
		if i < len(partsA) {
			fmt.Sscanf(partsA[i], "%d", &na)
		}
		if i < len(partsB) {
			fmt.Sscanf(partsB[i], "%d", &nb)
		}
		if na != nb {
			return na - nb
		}
	}
	return 0
}
