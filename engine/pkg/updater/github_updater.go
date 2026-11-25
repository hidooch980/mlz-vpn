// Package updater checks GitHub Releases / CDN version manifests, verifies SHA-256 checksums,
// and exposes Persian release notes for in-app updates.
package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
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
	// Signature is an Ed25519 signature over the canonical manifest without
	// this field. Release builds require it so a tampered version.json cannot
	// advertise an attacker-controlled download.
	Signature string `json:"signature,omitempty"`
}

// CheckResult tells the Flutter UI whether an update is available and where to fetch it.
type CheckResult struct {
	UpdateAvailable bool             `json:"update_available"`
	CurrentBuild    int              `json:"current_build"`
	Manifest        *VersionManifest `json:"manifest,omitempty"`
	AssetKey        string           `json:"asset_key,omitempty"`
}

// CheckForUpdate is the build-number-only variant kept for compatibility.
func CheckForUpdate(ctx context.Context, currentBuild int, manifestURLs []string, localProxyPort int) (CheckResult, error) {
	return CheckForUpdateVersion(ctx, currentBuild, "", manifestURLs, localProxyPort)
}

// CheckForUpdateVersion compares both the monotonic build number and the semantic
// version, so a release pipeline that forgets to bump one still cannot offer a
// downgrade or a same-version "update".
func CheckForUpdateVersion(ctx context.Context, currentBuild int, currentVersion string, manifestURLs []string, localProxyPort int) (CheckResult, error) {
	return CheckForUpdateVersionSigned(ctx, currentBuild, currentVersion, manifestURLs, localProxyPort, "")
}

// CheckForUpdateVersionSigned additionally verifies the Ed25519 manifest
// signature when publicKeyBase64 is non-empty. Shipping builds pass the MLZ
// release public key; an unsigned manifest is then rejected.
func CheckForUpdateVersionSigned(ctx context.Context, currentBuild int, currentVersion string, manifestURLs []string, localProxyPort int, publicKeyBase64 string) (CheckResult, error) {
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
		if publicKeyBase64 != "" {
			if err := VerifyManifestSignature(publicKeyBase64, &m); err != nil {
				lastErr = err
				continue
			}
		}

		return CheckResult{
			UpdateAvailable: m.BuildNumber > currentBuild || IsNewerVersion(m.Version, currentVersion),
			CurrentBuild:    currentBuild,
			Manifest:        &m,
			AssetKey:        PlatformAssetKey(),
		}, nil
	}
	return CheckResult{}, lastErr
}

// CanonicalManifestBytes serializes the manifest with the signature field
// cleared, which is what the release job signs.
func CanonicalManifestBytes(m *VersionManifest) ([]byte, error) {
	copy := *m
	copy.Signature = ""
	return json.Marshal(copy)
}

// VerifyManifestSignature verifies the Ed25519 signature over the canonical
// manifest. It rejects missing keys, malformed base64, and missing signatures.
func VerifyManifestSignature(publicKeyBase64 string, m *VersionManifest) error {
	if m == nil {
		return fmt.Errorf("update manifest is missing")
	}
	if strings.TrimSpace(m.Signature) == "" {
		return fmt.Errorf("update manifest is not signed")
	}
	pubRaw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(publicKeyBase64))
	if err != nil || len(pubRaw) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid update public key")
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(m.Signature))
	if err != nil || len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("invalid update manifest signature encoding")
	}
	raw, err := CanonicalManifestBytes(m)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(pubRaw), raw, sig) {
		return fmt.Errorf("update manifest signature mismatch")
	}
	return nil
}

// IsNewerVersion reports whether remote is strictly newer than current using
// numeric dotted segments. A missing or malformed side is treated as older.
func IsNewerVersion(remote, current string) bool {
	remote = strings.TrimSpace(strings.TrimPrefix(remote, "v"))
	current = strings.TrimSpace(strings.TrimPrefix(current, "v"))
	if remote == "" || current == "" {
		return false
	}
	rp := strings.SplitN(remote, "-", 2)[0]
	cp := strings.SplitN(current, "-", 2)[0]
	rSeg := strings.Split(rp, ".")
	cSeg := strings.Split(cp, ".")
	n := len(rSeg)
	if len(cSeg) > n {
		n = len(cSeg)
	}
	for i := 0; i < n; i++ {
		r, rOK := versionSegment(rSeg, i)
		c, cOK := versionSegment(cSeg, i)
		if !rOK && !cOK {
			return false
		}
		if r != c {
			return r > c
		}
	}
	return false
}

func versionSegment(parts []string, i int) (int, bool) {
	if i >= len(parts) {
		return 0, false
	}
	digits := strings.TrimSpace(parts[i])
	if digits == "" {
		return 0, false
	}
	n := 0
	for _, r := range digits {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

// PlatformAssetKey maps runtime OS/Arch to the version.json asset identifier.
func PlatformAssetKey() string {
	if runtime.GOOS == "android" {
		switch runtime.GOARCH {
		case "arm64":
			return "android_arm64"
		case "amd64":
			return "android_x86_64"
		case "386":
			return "android_x86"
		default:
			return "android_armv7"
		}
	}
	if runtime.GOOS == "windows" {
		if runtime.GOARCH == "arm64" {
			return "windows_arm64_setup"
		}
		// Prefer the installer so the in-app update can run it directly; the
		// portable ZIP remains available as a manual download on the release page.
		return "windows_x64_setup"
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
	const maxUpdateBytes int64 = 512 << 20
	if resp.ContentLength > maxUpdateBytes {
		return fmt.Errorf("update is too large: %d bytes", resp.ContentLength)
	}

	tmp := destPath + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	h := sha256.New()
	written, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, maxUpdateBytes+1))
	if err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if written > maxUpdateBytes {
		f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("update exceeds the %d-byte limit", maxUpdateBytes)
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
