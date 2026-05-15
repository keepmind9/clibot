package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	updateRepo      = "keepmind9/clibot"
	httpTimeout     = 30 * time.Second
	downloadTimeout = 10 * time.Minute
)

var (
	updateAPIURL = "https://api.github.com/repos/" + updateRepo + "/releases/latest"
	updateClient = &http.Client{}
)

var errDelayedSwap = errors.New("delayed swap scheduled")

type updateMeta struct {
	Version    string `json:"version"`
	URL        string `json:"url"`
	Size       int64  `json:"size"`
	Archive    string `json:"archive"`
	BinaryName string `json:"binary_name"`
}

func newUpdateCmd() *cobra.Command {
	var apply bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for updates and download the latest version",
		Long:  "Check for the latest version on GitHub and download it with resume support. Use --apply to replace the current binary.",
		RunE: func(_ *cobra.Command, _ []string) error {
			if apply {
				return runApply()
			}
			return runUpdate()
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "apply a downloaded update (replace binary)")
	return cmd
}

func runUpdate() error {
	latest, err := fetchLatestVersion()
	if err != nil {
		return fmt.Errorf("failed to fetch latest version: %w", err)
	}

	if !isNewerVersion(latest, Version) {
		fmt.Printf("Already on the latest version: %s\n", Version)
		return nil
	}

	fmt.Printf("New version available: %s (current: %s)\n", latest, Version)

	updateDir, err := ensureUpdateDir()
	if err != nil {
		return err
	}

	metaPath := filepath.Join(updateDir, "meta.json")
	archiveName := buildArchiveName(latest)
	archiveURL := buildArchiveURL(latest, archiveName)

	// Check existing meta for resume or reset
	var totalSize int64
	if meta, err := loadMeta(metaPath); err == nil && meta.Version == latest && meta.Archive != "" {
		fmt.Println("Resuming download...")
		totalSize = meta.Size
	} else {
		cleanUpdateDir(updateDir)
		totalSize, err = getRemoteSize(archiveURL)
		if err != nil {
			return fmt.Errorf("failed to get remote file size: %w", err)
		}
		binName := binaryName()
		if err := saveMeta(metaPath, &updateMeta{
			Version:    latest,
			URL:        archiveURL,
			Size:       totalSize,
			Archive:    archiveName,
			BinaryName: binName,
		}); err != nil {
			return err
		}
	}

	// Download with resume support
	archivePath := filepath.Join(updateDir, archiveName)
	if err := downloadWithResume(archiveURL, archivePath, totalSize); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	fmt.Println("Download complete. Extracting...")

	// Extract archive
	if err := extractArchive(updateDir, archivePath); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Verify extraction before removing archive
	binPath, err := findExtractedBinary(updateDir)
	if err != nil {
		return fmt.Errorf("extraction failed or binary not found: %w\nThe archive may be corrupted.", err)
	}

	// Remove archive after successful extraction
	os.Remove(archivePath)
	fmt.Printf("Binary extracted to: %s\n", binPath)

	if !isProcessRunning() {
		fmt.Println("Applying update...")
		if err := doApply(updateDir); err != nil {
			return fmt.Errorf("%w\n\nTry: clibot update --apply", err)
		}
		return nil
	}

	fmt.Println("Run 'clibot update --apply' to apply the update.")
	return nil
}

func runApply() error {
	updateDir, err := updateDirPath()
	if err != nil {
		return err
	}

	metaPath := filepath.Join(updateDir, "meta.json")
	meta, err := loadMeta(metaPath)
	if err != nil {
		fmt.Println("No update available. Run 'clibot update' first.")
		return nil
	}

	// Verify binary exists and is valid
	binPath, err := findExtractedBinary(updateDir)
	if err != nil {
		fmt.Println("Downloaded file is incomplete or corrupted. Run 'clibot update' to re-download.")
		return nil
	}
	info, err := os.Stat(binPath)
	if err != nil || info.Size() == 0 {
		fmt.Println("Downloaded binary is empty or corrupted. Run 'clibot update' to re-download.")
		return nil
	}

	fmt.Printf("Applying update to %s...\n", meta.Version)
	return doApply(updateDir)
}

func doApply(updateDir string) error {
	metaPath := filepath.Join(updateDir, "meta.json")
	meta, err := loadMeta(metaPath)
	if err != nil {
		return err
	}

	// Find the extracted binary
	srcBin, err := findExtractedBinary(updateDir)
	if err != nil {
		return fmt.Errorf("binary not found: %w", err)
	}

	currentBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current binary path: %w", err)
	}

	// Resolve symlinks (e.g. /usr/local/bin/clibot -> actual binary)
	currentBin, err = filepath.EvalSymlinks(currentBin)
	if err != nil {
		return fmt.Errorf("failed to resolve binary path: %w", err)
	}

	if err := copyFile(srcBin, currentBin); err != nil {
		if errors.Is(err, errDelayedSwap) {
			return nil
		}
		return fmt.Errorf("failed to replace binary: %w", err)
	}
	if runtime.GOOS != "windows" {
		os.Chmod(currentBin, 0755)
	}

	cleanUpdateDir(updateDir)

	fmt.Printf("Successfully updated to %s\n", meta.Version)
	fmt.Println("Restart clibot to use the new version.")
	return nil
}

// --- Version ---

func fetchLatestVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, updateAPIURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := updateClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var result struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return strings.TrimPrefix(result.TagName, "v"), nil
}

func isNewerVersion(latest, current string) bool {
	return compareVersions(latest, current) > 0
}

func compareVersions(a, b string) int {
	pa := parseVersion(a)
	pb := parseVersion(b)
	for i := range pa {
		if pa[i] != pb[i] {
			if pa[i] > pb[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

func parseVersion(v string) [3]int {
	var major, minor, patch int
	parts := strings.Split(v, ".")
	if len(parts) >= 1 {
		fmt.Sscanf(parts[0], "%d", &major)
	}
	if len(parts) >= 2 {
		fmt.Sscanf(parts[1], "%d", &minor)
	}
	if len(parts) >= 3 {
		fmt.Sscanf(parts[2], "%d", &patch)
	}
	return [3]int{major, minor, patch}
}

// --- Archive ---

func buildArchiveName(latest string) string {
	name := fmt.Sprintf("clibot-%s-%s-%s", latest, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		return name + ".zip"
	}
	return name + ".tar.gz"
}

func buildArchiveURL(latest, archive string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/v%s/%s", updateRepo, latest, archive)
}

func binaryName() string {
	name := "clibot"
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func getRemoteSize(url string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := updateClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HEAD returned %d", resp.StatusCode)
	}
	return resp.ContentLength, nil
}

func downloadWithResume(url, dest string, totalSize int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()

	var offset int64
	if info, err := os.Stat(dest); err == nil {
		offset = info.Size()
		if offset == totalSize {
			return nil // already complete
		}
	}

	// Resume if partial file exists
	if offset > 0 {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))

		resp, err := updateClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusPartialContent {
			f, err := os.OpenFile(dest, os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(f, resp.Body)
			return err
		}

		resp.Body.Close()
		os.Remove(dest)
	}

	// Fresh download
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := updateClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	_, err = io.Copy(f, resp.Body)
	return err
}

func extractArchive(dest, archive string) error {
	if runtime.GOOS == "windows" {
		return extractZip(dest, archive)
	}
	return extractTarGz(dest, archive)
}

func extractTarGz(dest, archive string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Only extract regular files
		if header.Typeflag != tar.TypeReg {
			continue
		}

		targetPath := filepath.Join(dest, header.Name)
		// Path traversal check
		cleanPath := filepath.Clean(targetPath)
		if !strings.HasPrefix(cleanPath, filepath.Clean(dest)+string(filepath.Separator)) {
			continue // skip malicious entries
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		outFile, err := os.Create(targetPath)
		if err != nil {
			return err
		}
		if _, err := io.Copy(outFile, tarReader); err != nil {
			outFile.Close()
			return err
		}
		outFile.Close()

		// Preserve executable permission
		os.Chmod(targetPath, os.FileMode(header.Mode))
	}
	return nil
}

func extractZip(dest, archive string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		// Only extract regular files
		if !file.Mode().IsRegular() {
			continue
		}

		targetPath := filepath.Join(dest, file.Name)
		// Path traversal check
		cleanPath := filepath.Clean(targetPath)
		if !strings.HasPrefix(cleanPath, filepath.Clean(dest)+string(filepath.Separator)) {
			continue // skip malicious entries
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		outFile, err := os.Create(targetPath)
		if err != nil {
			return err
		}

		rc, err := file.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		if _, err := io.Copy(outFile, rc); err != nil {
			rc.Close()
			outFile.Close()
			return err
		}
		rc.Close()
		outFile.Close()
	}
	return nil
}

func findExtractedBinary(dir string) (string, error) {
	name := binaryName()
	prefix := "clibot-"

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		candidate := filepath.Join(dir, entry.Name(), name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("binary not found in %s", dir)
}

// --- Meta ---

func updateDirPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".clibot", "update"), nil
}

func ensureUpdateDir() (string, error) {
	path, err := updateDirPath()
	if err != nil {
		return "", err
	}
	return path, os.MkdirAll(path, 0755)
}

func loadMeta(path string) (*updateMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var meta updateMeta
	return &meta, json.Unmarshal(data, &meta)
}

func saveMeta(path string, meta *updateMeta) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func cleanUpdateDir(dir string) {
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
}

// --- Process check ---

func isProcessRunning() bool {
	pid := os.Getpid()
	pgrep, err := exec.LookPath("pgrep")
	if err != nil {
		return false
	}
	out, _ := exec.Command(pgrep, "-x", "clibot").Output()
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == fmt.Sprintf("%d", pid) {
			continue
		}
		if strings.TrimSpace(line) != "" {
			return true
		}
	}
	return false
}

// --- Copy ---

func copyFile(src, dst string) error {
	if runtime.GOOS == "windows" {
		return copyFileWindows(src, dst)
	}
	return copyFileUnix(src, dst)
}

func copyFileUnix(src, dst string) error {
	// Rename the old binary first to avoid "text file busy" when overwriting
	// a running binary. On Unix, an open file can be renamed but not truncated.
	backup := dst + ".old"
	renamed := false
	if _, err := os.Stat(dst); err == nil {
		if err := os.Rename(dst, backup); err != nil {
			return fmt.Errorf("failed to rename current binary: %w", err)
		}
		renamed = true
	}

	in, err := os.Open(src)
	if err != nil {
		if renamed {
			os.Rename(backup, dst)
		}
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		if renamed {
			os.Rename(backup, dst)
		}
		return fmt.Errorf("failed to create binary: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		if renamed {
			os.Rename(backup, dst)
		}
		return fmt.Errorf("failed to write binary: %w", err)
	}

	if renamed {
		os.Remove(backup)
	}
	return nil
}

func copyFileWindows(src, dst string) error {
	// On Windows, a running exe is fully locked — rename and write both fail.
	// Write to a .new file and schedule a delayed swap via a helper script.
	tmpDst := dst + ".new"

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(tmpDst)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	out.Close()

	// Try direct rename first (works if the exe is not running)
	if err := os.Rename(tmpDst, dst); err == nil {
		return nil
	}

	// Direct rename failed — schedule a delayed swap via a helper script.
	script := fmt.Sprintf("@echo off\r\n:wait\r\ntasklist /fi \"pid eq %d\" 2>nul | find \"%d\" >nul\r\nif not errorlevel 1 (\r\n  timeout /t 1 /nobreak >nul\r\n  goto wait\r\n)\r\nmove /y \"%s\" \"%s\"\r\ndel \"%%~f0\"\r\n",
		os.Getpid(), os.Getpid(), tmpDst, dst)
	scriptPath := dst + ".update.cmd"
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		return fmt.Errorf("failed to create update script: %w", err)
	}

	cmd := exec.Command("cmd", "/c", "start", "/b", "", scriptPath)
	if err := cmd.Start(); err != nil {
		os.Remove(scriptPath)
		return fmt.Errorf("failed to schedule update: %w\n\nManual steps:\n  1. Close this terminal\n  2. Run: move /y \"%s\" \"%s\"", err, tmpDst, dst)
	}

	fmt.Println("Update will complete automatically after this process exits.")
	fmt.Println("Please close this terminal and start clibot again.")
	return errDelayedSwap
}
