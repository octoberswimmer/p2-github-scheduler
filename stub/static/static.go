// Package static is a stub that replaces the real static package
// which contains generated web assets not needed by p2-github-scheduler.
package static

import (
	"crypto/sha256"
	"fmt"
	"os"
)

func Asset(name string) ([]byte, error) {
	return nil, fmt.Errorf("static assets not available in p2-github-scheduler")
}

func AssetString(name string) (string, error) {
	return "", fmt.Errorf("static assets not available in p2-github-scheduler")
}

func MustAsset(name string) []byte {
	panic("static assets not available in p2-github-scheduler")
}

func MustAssetString(name string) string {
	panic("static assets not available in p2-github-scheduler")
}

func AssetInfo(name string) (os.FileInfo, error) {
	return nil, fmt.Errorf("static assets not available in p2-github-scheduler")
}

func AssetDigest(name string) ([sha256.Size]byte, error) {
	return [sha256.Size]byte{}, fmt.Errorf("static assets not available in p2-github-scheduler")
}

func Digests() (map[string][sha256.Size]byte, error) {
	return nil, fmt.Errorf("static assets not available in p2-github-scheduler")
}

func AssetNames() []string {
	return nil
}

func AssetDir(name string) ([]string, error) {
	return nil, fmt.Errorf("static assets not available in p2-github-scheduler")
}

func RestoreAsset(dir, name string) error {
	return fmt.Errorf("static assets not available in p2-github-scheduler")
}

func RestoreAssets(dir, name string) error {
	return fmt.Errorf("static assets not available in p2-github-scheduler")
}
