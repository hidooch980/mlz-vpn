package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

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

	res, err := CheckForUpdate(context.Background(), 100, []string{srv.URL + "/version.json"}, 0)
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
