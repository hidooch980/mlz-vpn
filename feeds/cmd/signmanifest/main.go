// Command signmanifest signs version.json with the MLZ Ed25519 release key so
// clients can reject tampered update manifests.
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/updater"
)

func main() {
	path := flag.String("file", "version.json", "version manifest to sign in place")
	flag.Parse()

	privB64 := strings.TrimSpace(os.Getenv("MLZ_FEED_PRIVKEY"))
	raw, err := base64.StdEncoding.DecodeString(privB64)
	if privB64 == "" || err != nil || len(raw) != ed25519.PrivateKeySize {
		fmt.Fprintln(os.Stderr, "MLZ_FEED_PRIVKEY is required and must be an Ed25519 private key")
		os.Exit(1)
	}

	data, err := os.ReadFile(*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", *path, err)
		os.Exit(1)
	}
	var manifest updater.VersionManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "parse %s: %v\n", *path, err)
		os.Exit(1)
	}

	canonical, err := updater.CanonicalManifestBytes(&manifest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "canonicalize: %v\n", err)
		os.Exit(1)
	}
	manifest.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(ed25519.PrivateKey(raw), canonical))

	out, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*path, out, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}
}
