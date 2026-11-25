// Package updater checks GitHub Releases / CDN version manifests, verifies SHA-256 checksums,
// and exposes Persian release notes for in-app updates.
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"
)

// AssetInfo describes a downloadable platform release artifact.
type AssetInfo struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// VersionManifest matches the version.json file published on GitHub Releases.
type VersionManifest struct {
	Version             string               `json:"version"`
	BuildNumber         int                  `json:"build_number"`
	MinSupportedVersion string               `json:"min_supported_version"`
	ReleasedAt          string               `json:"released_at"`
	ChangelogFA         []string             `json:"changelog_fa"`
	Assets              map[string]AssetInfo `json:"assets"`
}

// CheckResult tells the Flutter UI whether an update is available and where to fetch it.
type CheckResult struct {
	UpdateAvailable bool             `json:"update_available"`
	CurrentBuild    int              `json:"current_build"`
	Manifest        *VersionManifest `json:"manifest,omitempty"`
	AssetKey        string           `json:"asset_key,omitempty"`
}

// CheckForUpdate fetches version.json via active proxy (if connected) or mirror URLs.
func CheckForUpdate(ctx context.Context, currentBuild int, manifestURLs []string, localProxyPort int) (CheckResult, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	if localProxyPort > 0 {
		if u, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", localProxyPort)); err == nil {
			client.Transport = &http.Transport{Proxy: http.ProxyURL(u)}
		}
	}

	var lastErr error
	for _, mURL := range manifestURLs {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, mURL, nil)
		if err != nil {
			lastErr = err
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			continue
		}

		var m VersionManifest
		decErr := json.NewDecoder(io.LimitReader(resp.Body, 512*1024)).Decode(&m)
		resp.Body.Close()
		if decErr != nil {
			lastErr = decErr
			continue
		}

		return CheckResult{
			UpdateAvailable: m.BuildNumber > currentBuild,
			CurrentBuild:    currentBuild,
			Manifest:        &m,
			AssetKey:        PlatformAssetKey(),
		}, nil
	}
	return CheckResult{}, lastErr
}

// PlatformAssetKey maps runtime OS/Arch to the version.json asset identifier.
func PlatformAssetKey() string {
	if runtime.GOOS == "android" {
		if runtime.GOARCH == "arm64" {
			return "android_arm64"
		}
		return "android_universal"
	}
	if runtime.GOOS == "windows" {
		if runtime.GOARCH == "arm64" {
			return "windows_arm64"
		}
		return "windows_x64"
	}
	return runtime.GOOS + "_" + runtime.GOARCH
}

// DownloadAndVerify downloads a release installer/APK and checks its SHA-256 digest before installing.
func DownloadAndVerify(ctx context.Context, asset AssetInfo, destPath string, localProxyPort int) error {
	client := &http.Client{Timeout: 3 * time.Minute}
	if localProxyPort > 0 {
		if u, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", localProxyPort)); err == nil {
			client.Transport = &http.Transport{Proxy: http.ProxyURL(u)}
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	tmp := destPath + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, strings.TrimSpace(asset.SHA256)) {
		_ = os.Remove(tmp)
		return fmt.Errorf("update SHA-256 mismatch: got %s expected %s", got, asset.SHA256)
	}
	return os.Rename(tmp, destPath)
}
