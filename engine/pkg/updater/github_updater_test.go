package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		remote, current string
		want            bool
	}{
		{"1.2.0", "1.0.0", true},
		{"v1.0.1", "1.0.0", true},
		{"1.0.0", "1.0.0", false},
		{"1.0", "1.0.0", false},
		{"1.0.0", "1.0", false},
		{"2.0", "10.0", false},
		{"1.10.0", "1.9.9", true},
		{"1.0.0-beta", "1.0.0", false},
		{"", "1.0.0", false},
	}
	for _, tc := range cases {
		if got := IsNewerVersion(tc.remote, tc.current); got != tc.want {
			t.Fatalf("IsNewerVersion(%q,%q)=%v want %v", tc.remote, tc.current, got, tc.want)
		}
	}
}

func TestVerifyManifestSignature(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	m := &VersionManifest{
		Version:     "1.2.0",
		BuildNumber: 200,
		ChangelogFA: []string{"تست"},
	}
	canonical, err := CanonicalManifestBytes(m)
	if err != nil {
		t.Fatal(err)
	}
	m.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, canonical))
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	if err := VerifyManifestSignature(pubB64, m); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}

	tampered := *m
	tampered.Version = "1.2.1"
	if err := VerifyManifestSignature(pubB64, &tampered); err == nil {
		t.Fatal("tampered manifest accepted")
	}
	unsigned := *m
	unsigned.Signature = ""
	if err := VerifyManifestSignature(pubB64, &unsigned); err == nil {
		t.Fatal("unsigned manifest accepted")
	}
}

func TestCheckForUpdateAndVerifyDownload(t *testing.T) {
	payload := []byte("mlz-vpn-binary-test-payload")
	sum := sha256.Sum256(payload)
	hexSum := hex.EncodeToString(sum[:])

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/version.json":
			_ = json.NewEncoder(w).Encode(VersionManifest{
				Version:     "1.2.0",
				BuildNumber: 200,
				ChangelogFA: []string{"بهبود سرعت اتصال"},
				Assets: map[string]AssetInfo{
					PlatformAssetKey(): {
						URL:    srv.URL + "/asset.bin",
						SHA256: hexSum,
					},
				},
			})
		case "/asset.bin":
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	res, err := CheckForUpdateVersion(context.Background(), 100, "1.0.0", []string{srv.URL + "/version.json"}, 0)
	if err != nil {
		t.Fatalf("CheckForUpdate error: %v", err)
	}
	if !res.UpdateAvailable || res.Manifest.Version != "1.2.0" {
		t.Fatalf("expected update available v1.2.0, got %+v", res)
	}

	dest := filepath.Join(t.TempDir(), "asset.bin")
	if err := DownloadAndVerify(context.Background(), res.Manifest.Assets[res.AssetKey], dest, 0); err != nil {
		t.Fatalf("DownloadAndVerify error: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != string(payload) {
		t.Fatalf("downloaded file content mismatch: %v", err)
	}

	badAsset := AssetInfo{URL: srv.URL + "/asset.bin", SHA256: "0000"}
	if err := DownloadAndVerify(context.Background(), badAsset, dest+".bad", 0); err == nil {
		t.Fatal("expected SHA-256 mismatch error")
	}
}
