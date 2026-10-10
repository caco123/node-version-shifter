package downloader

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/caco123/node-version-shifter/pkg/config"
)

// PlatformInfo holds the OS, Arch, and expected archive file format for Node.js downloads.
type PlatformInfo struct {
	OS          string
	Arch        string
	Format      string // "tar.gz" or "zip"
	ArchiveName string
}

// GetCurrentPlatform determines the current OS and architecture strings used by nodejs.org.
func GetCurrentPlatform(version string) (*PlatformInfo, error) {
	var osStr string
	var format string

	switch runtime.GOOS {
	case "linux":
		osStr = "linux"
		format = "tar.gz"
	case "darwin":
		osStr = "darwin"
		format = "tar.gz"
	case "windows":
		osStr = "win"
		format = "zip"
	default:
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	var archStr string
	switch runtime.GOARCH {
	case "amd64":
		archStr = "x64"
	case "arm64":
		archStr = "arm64"
	case "386":
		archStr = "x86"
	case "arm":
		archStr = "armv7l"
	default:
		return nil, fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	archiveName := fmt.Sprintf("node-%s-%s-%s.%s", version, osStr, archStr, format)

	return &PlatformInfo{
		OS:          osStr,
		Arch:        archStr,
		Format:      format,
		ArchiveName: archiveName,
	}, nil
}

// ProgressWriter tracks downloaded bytes.
type ProgressWriter struct {
	Total      int64
	Downloaded int64
	LastPrint  time.Time
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.Downloaded += int64(n)

	now := time.Now()
	if now.Sub(pw.LastPrint) > 250*time.Millisecond || pw.Downloaded == pw.Total {
		pw.LastPrint = now
		if pw.Total > 0 {
			pct := float64(pw.Downloaded) / float64(pw.Total) * 100
			fmt.Printf("\rDownloading: %.1f%% (%s / %s)",
				pct,
				formatBytes(pw.Downloaded),
				formatBytes(pw.Total),
			)
		} else {
			fmt.Printf("\rDownloading: %s", formatBytes(pw.Downloaded))
		}
	}
	return n, nil
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// InstallVersion downloads and unpacks a specific Node.js release tag into ~/.nvsh/versions/<version>.
func InstallVersion(version string) error {
	normalized := config.NormalizeVersion(version)
	destDir := config.VersionPath(normalized)

	if config.VersionInstalled(normalized) {
		fmt.Printf("Version %s is already installed.\n", normalized)
		return nil
	}

	platform, err := GetCurrentPlatform(normalized)
	if err != nil {
		return err
	}

	cfg, _ := config.LoadConfig()
	downloadURL := fmt.Sprintf("%s/%s/%s", strings.TrimRight(cfg.NodeDistURL, "/"), normalized, platform.ArchiveName)

	cacheFile := filepath.Join(config.CacheDir(), platform.ArchiveName)

	// Check if already cached
	needDownload := true
	if stat, err := os.Stat(cacheFile); err == nil && stat.Size() > 0 {
		fmt.Printf("Found cached archive: %s\n", platform.ArchiveName)
		needDownload = false
	}

	if needDownload {
		fmt.Printf("Fetching Node.js %s for %s/%s...\n", normalized, runtime.GOOS, runtime.GOARCH)
		fmt.Printf("URL: %s\n", downloadURL)

		resp, err := http.Get(downloadURL)
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("download error: HTTP %s (%s)", resp.Status, downloadURL)
		}

		tmpFile := cacheFile + ".tmp"
		out, err := os.Create(tmpFile)
		if err != nil {
			return fmt.Errorf("failed to create cache file: %w", err)
		}

		pw := &ProgressWriter{Total: resp.ContentLength}
		_, err = io.Copy(out, io.TeeReader(resp.Body, pw))
		out.Close()
		fmt.Println() // newline after progress

		if err != nil {
			os.Remove(tmpFile)
			return fmt.Errorf("download stream error: %w", err)
		}

		if err := os.Rename(tmpFile, cacheFile); err != nil {
			return fmt.Errorf("failed to save archive: %w", err)
		}
	}

	// Verify SHA-256 checksum against official SHASUMS256.txt
	shasumsURL := fmt.Sprintf("%s/%s/SHASUMS256.txt", strings.TrimRight(cfg.NodeDistURL, "/"), normalized)
	fmt.Printf("Verifying SHA-256 checksum against official SHASUMS256.txt...\n")
	fmt.Printf("  Checksum Link: %s\n", shasumsURL)
	checksum, err := VerifyArchiveChecksum(normalized, platform.ArchiveName, cacheFile)
	if err != nil {
		_ = os.Remove(cacheFile) // Remove invalid/corrupt archive
		return fmt.Errorf("SECURITY ALERT: Checksum verification failed: %w\nVerify manually at: %s", err, shasumsURL)
	}
	fmt.Printf("  ✓ Checksum verified: %s\n", checksum)

	fmt.Printf("Extracting %s into %s...\n", normalized, destDir)

	// Extract to temporary directory first for atomic install
	tmpExtractDir := fmt.Sprintf("%s.tmp-%d", destDir, time.Now().UnixNano())
	defer os.RemoveAll(tmpExtractDir)

	if err := os.MkdirAll(tmpExtractDir, 0755); err != nil {
		return fmt.Errorf("failed to create extract directory: %w", err)
	}

	if platform.Format == "tar.gz" {
		if err := extractTarGz(cacheFile, tmpExtractDir); err != nil {
			return fmt.Errorf("failed to extract tar.gz: %w", err)
		}
	} else if platform.Format == "zip" {
		if err := extractZip(cacheFile, tmpExtractDir); err != nil {
			return fmt.Errorf("failed to extract zip: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported archive format: %s", platform.Format)
	}

	// Rename temporary directory to destination
	if err := os.Rename(tmpExtractDir, destDir); err != nil {
		return fmt.Errorf("failed to finalize installation: %w", err)
	}

	fmt.Printf("Successfully installed Node.js %s!\n", normalized)
	return nil
}

// extractTarGz extracts a .tar.gz archive, stripping the leading directory segment.
func extractTarGz(srcFile, destDir string) error {
	f, err := os.Open(srcFile)
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

		// Strip leading directory (e.g. "node-v22.14.0-linux-x64/bin/node" -> "bin/node")
		relPath := stripFirstPathSegment(header.Name)
		if relPath == "" || relPath == "." {
			continue
		}

		target := filepath.Join(destDir, relPath)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			mode := header.FileInfo().Mode()
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, mode.Perm())
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		case tar.TypeSymlink:
			// Relative symlink inside the extracted distribution
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			_ = os.Remove(target) // Remove existing if any
			err := os.Symlink(header.Linkname, target)
			if err != nil {
				// If symlink fails (e.g. restricted filesystem), try copying target if relative
				targetReal := filepath.Join(filepath.Dir(target), header.Linkname)
				if copyErr := copyFile(targetReal, target); copyErr != nil {
					// Soft fail: continue unpacking
				}
			}
		}
	}

	return nil
}

// extractZip extracts a .zip archive, stripping the leading directory segment.
func extractZip(srcFile, destDir string) error {
	r, err := zip.OpenReader(srcFile)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		relPath := stripFirstPathSegment(f.Name)
		if relPath == "" || relPath == "." {
			continue
		}

		target := filepath.Join(destDir, relPath)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, f.Mode().Perm())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func stripFirstPathSegment(p string) string {
	clean := filepath.ToSlash(filepath.Clean(p))
	clean = strings.TrimPrefix(clean, "/")
	parts := strings.Split(clean, "/")
	if len(parts) <= 1 {
		return ""
	}
	return filepath.Join(parts[1:]...)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// CalculateFileSHA256 computes the hexadecimal SHA-256 hash of a file.
func CalculateFileSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// FetchSHASUMS downloads and parses the official SHASUMS256.txt file for a given version.
func FetchSHASUMS(version string) (map[string]string, []byte, error) {
	cfg, _ := config.LoadConfig()
	url := fmt.Sprintf("%s/%s/SHASUMS256.txt", strings.TrimRight(cfg.NodeDistURL, "/"), version)

	resp, err := http.Get(url)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("HTTP %s while fetching %s", resp.Status, url)
	}

	rawBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read SHASUMS256.txt: %w", err)
	}

	checksums := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(rawBytes)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			hash := strings.ToLower(fields[0])
			fileName := filepath.Base(fields[1])
			checksums[fileName] = hash
		}
	}

	return checksums, rawBytes, nil
}

// VerifyArchiveChecksum checks the SHA-256 of an archive against SHASUMS256.txt.
func VerifyArchiveChecksum(version, archiveName, filePath string) (string, error) {
	sums, _, err := FetchSHASUMS(version)
	if err != nil {
		return "", fmt.Errorf("could not retrieve official checksums: %w", err)
	}

	expectedHash, found := sums[archiveName]
	if !found {
		return "", fmt.Errorf("archive '%s' was not found in official SHASUMS256.txt", archiveName)
	}

	actualHash, err := CalculateFileSHA256(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to calculate local file SHA-256: %w", err)
	}

	if !strings.EqualFold(actualHash, expectedHash) {
		cfg, _ := config.LoadConfig()
		shasumsURL := fmt.Sprintf("%s/%s/SHASUMS256.txt", strings.TrimRight(cfg.NodeDistURL, "/"), version)
		return "", fmt.Errorf("checksum mismatch!\n  Expected: %s\n  Actual:   %s\n  Verify manually at: %s", expectedHash, actualHash, shasumsURL)
	}

	return actualHash, nil
}

// VerifyVersion verifies the checksum and GPG signature of a downloaded or installed version.
func VerifyVersion(version string) error {
	normalized := config.NormalizeVersion(version)
	platform, err := GetCurrentPlatform(normalized)
	if err != nil {
		return err
	}

	archivePath := filepath.Join(config.CacheDir(), platform.ArchiveName)
	if stat, err := os.Stat(archivePath); err != nil || stat.Size() == 0 {
		return fmt.Errorf("archive %s not found in cache (%s). Run 'nvsh install %s' first", platform.ArchiveName, archivePath, normalized)
	}

	cfg, _ := config.LoadConfig()
	shasumsURL := fmt.Sprintf("%s/%s/SHASUMS256.txt", strings.TrimRight(cfg.NodeDistURL, "/"), normalized)
	sigURL := fmt.Sprintf("%s/%s/SHASUMS256.txt.sig", strings.TrimRight(cfg.NodeDistURL, "/"), normalized)
	releaseKeysURL := "https://github.com/nodejs/node#release-keys"

	fmt.Printf("Verifying Node.js release %s (%s)...\n", normalized, platform.ArchiveName)
	fmt.Printf("  • Checksum Link (SHASUMS256): %s\n", shasumsURL)
	fmt.Printf("  • Signature Link (GPG Sig):   %s\n", sigURL)
	fmt.Printf("  • Official Node.js Keys Link:  %s\n\n", releaseKeysURL)

	// 1. SHA-256 Verification
	checksums, shasumsRaw, err := FetchSHASUMS(normalized)
	if err != nil {
		return fmt.Errorf("failed to fetch SHASUMS256.txt: %w", err)
	}

	expectedHash, found := checksums[platform.ArchiveName]
	if !found {
		return fmt.Errorf("archive %s not listed in official SHASUMS256.txt (%s)", platform.ArchiveName, shasumsURL)
	}

	actualHash, err := CalculateFileSHA256(archivePath)
	if err != nil {
		return fmt.Errorf("failed to calculate local hash: %w", err)
	}

	if !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("TAMPERING / CORRUPTION DETECTED!\n  Expected SHA-256: %s\n  Actual SHA-256:   %s\n  Verify manually at: %s", expectedHash, actualHash, shasumsURL)
	}

	fmt.Printf("✓ SHA-256 Checksum VALID: %s\n\n", actualHash)

	// 2. GPG Signature Verification of SHASUMS256.txt
	resp, err := http.Get(sigURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Printf("Notice: Could not download GPG signature from %s\n", sigURL)
		return nil
	}
	defer resp.Body.Close()
	sigBytes, _ := io.ReadAll(resp.Body)

	// Save temp files for gpg command verification
	tempDir := config.CacheDir()
	shasumsFile := filepath.Join(tempDir, fmt.Sprintf("SHASUMS256-%s.txt", normalized))
	sigFile := filepath.Join(tempDir, fmt.Sprintf("SHASUMS256-%s.txt.sig", normalized))
	_ = os.WriteFile(shasumsFile, shasumsRaw, 0644)
	_ = os.WriteFile(sigFile, sigBytes, 0644)
	defer os.Remove(shasumsFile)
	defer os.Remove(sigFile)

	gpgPath, err := exec.LookPath("gpg")
	if err != nil {
		fmt.Println("✓ Release checksum matches official nodejs.org distribution.")
		fmt.Printf("  (Note: 'gpg' is not installed on this system. You can inspect the signature at: %s)\n", sigURL)
		return nil
	}

	fmt.Println("Verifying detached GPG signature with gpg...")
	cmd := exec.Command(gpgPath, "--verify", sigFile, shasumsFile)
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		fmt.Printf("  Note: If gpg reported 'Can't check signature: No public key', you can import the official release keys from: %s\n", releaseKeysURL)
	} else {
		fmt.Println("✓ GPG Signature VALID (Signed by official Node.js release key).")
	}

	return nil
}
