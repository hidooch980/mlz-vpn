// Command keygen creates the Ed25519 key pair used to sign feeds and update
// manifests. Store the private key only in CI secrets, never in the repository.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func main() {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Println("key generation failed:", err)
		return
	}
	fmt.Printf("MLZ_FEED_PRIVKEY=%s\n", base64.StdEncoding.EncodeToString(priv))
	fmt.Printf("MLZ_FEED_PUBKEY=%s\n", base64.StdEncoding.EncodeToString(pub))
}
