// updatesign creates and signs client update manifests.
//
// One-time key setup (keep the key file secret and backed up; clients only
// trust updates signed with it):
//
//	go run ./tools/updatesign -genkey
//
// Publishing (done by build.ps1 / build.sh):
//
//	go run ./tools/updatesign -platform windows-amd64 -version 0.7.3 \
//	    -file build/ProIdentity-Access-0.7.3.msi \
//	    -url /api/v1/client-updates/windows-amd64/ProIdentity-Access-0.7.3.msi \
//	    -out ../server/internal/api/client_updates/windows/latest.json
//
// The key is read from -key, $PROIDENTITY_UPDATE_KEY, or
// ~/.proidentity/update-signing.key (base64 Ed25519 seed).
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wg-client/internal/update"
)

func main() {
	genkey := flag.Bool("genkey", false, "generate a new signing key and print its public key")
	keyPath := flag.String("key", "", "signing key file (default $PROIDENTITY_UPDATE_KEY or ~/.proidentity/update-signing.key)")
	platform := flag.String("platform", "", "platform id, e.g. windows-amd64, darwin-arm64")
	version := flag.String("version", "", "package version, e.g. 0.7.3")
	file := flag.String("file", "", "installer file (.msi / .pkg)")
	url := flag.String("url", "", "download URL (absolute https, or server-relative path)")
	mandatory := flag.Bool("mandatory", false, "users cannot postpone this update")
	notes := flag.String("notes", "", "short release notes shown in the update prompt")
	out := flag.String("out", "", "manifest output path (latest.json)")
	flag.Parse()

	path := resolveKeyPath(*keyPath)
	if *genkey {
		must(generate(path))
		return
	}
	if *platform == "" || *version == "" || *file == "" || *url == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}

	priv, err := loadKey(path)
	must(err)

	f, err := os.Open(*file)
	must(err)
	h := sha256.New()
	size, err := io.Copy(h, f)
	f.Close()
	must(err)

	m := update.Manifest{
		Platform:      *platform,
		Version:       *version,
		LatestVersion: *version,
		FileName:      filepath.Base(*file),
		URL:           *url,
		SHA256:        hex.EncodeToString(h.Sum(nil)),
		Size:          size,
		PublishedAt:   time.Now().UTC().Format(time.RFC3339),
		Mandatory:     *mandatory,
		Notes:         *notes,
	}
	must(m.Validate())
	m.Sign(priv)
	must(m.VerifyWith(priv.Public().(ed25519.PublicKey)))

	data, err := json.MarshalIndent(m, "", "  ")
	must(err)
	must(os.MkdirAll(filepath.Dir(*out), 0o755))
	must(os.WriteFile(*out, append(data, '\n'), 0o644))
	fmt.Printf("Signed %s %s (%s, %d bytes) -> %s\n", m.Platform, m.Version, m.FileName, m.Size, *out)
}

func resolveKeyPath(flagPath string) string {
	if flagPath != "" {
		return flagPath
	}
	if env := os.Getenv("PROIDENTITY_UPDATE_KEY"); env != "" {
		return env
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".proidentity", "update-signing.key")
}

func generate(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists; refusing to overwrite the release signing key", path)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	seed := base64.StdEncoding.EncodeToString(priv.Seed())
	if err := os.WriteFile(path, []byte(seed+"\n"), 0o600); err != nil {
		return err
	}
	fmt.Printf("Private key written to %s. Back it up; it cannot be recovered.\n", path)
	fmt.Printf("Public key (set update.ReleasePublicKey):\n%s\n", base64.StdEncoding.EncodeToString(pub))
	return nil
}

func loadKey(path string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read signing key: %w (create one with -genkey)", err)
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("signing key %s is not a base64 Ed25519 seed", path)
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "updatesign:", err)
		os.Exit(1)
	}
}
