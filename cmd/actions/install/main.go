package main

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	var repo string
	var version string
	var runnerOS string
	var runnerArch string
	var dest string

	flag.StringVar(&repo, "repo", "", "repository that hosts the release assets")
	flag.StringVar(&version, "version", "", "release tag to download")
	flag.StringVar(&runnerOS, "runner-os", "", "runner operating system")
	flag.StringVar(&runnerArch, "runner-arch", "", "runner architecture")
	flag.StringVar(&dest, "dest", "", "destination directory for the binary")
	flag.Parse()

	if repo == "" || version == "" {
		log.Fatal("both --repo and --version are required")
	}

	runnerOS = strings.TrimSpace(runnerOS)
	runnerArch = strings.TrimSpace(runnerArch)

	if runnerOS == "" {
		runnerOS = runtime.GOOS
	}
	if runnerArch == "" {
		runnerArch = runtime.GOARCH
	}

	platformKey, err := normalizeOS(runnerOS)
	if err != nil {
		log.Fatal(err)
	}

	cpuKey, err := normalizeArch(runnerArch)
	if err != nil {
		log.Fatal(err)
	}

	// Archive naming: p2-github-scheduler_Linux_x86_64.tar.gz
	var archiveName string
	var binaryName string
	archKey := archiveArch(cpuKey)
	if platformKey == "windows" {
		archiveName = fmt.Sprintf("p2-github-scheduler_Windows_%s.zip", archKey)
		binaryName = "p2-github-scheduler.exe"
	} else {
		osKey := titleCase(platformKey)
		archiveName = fmt.Sprintf("p2-github-scheduler_%s_%s.tar.gz", osKey, archKey)
		binaryName = "p2-github-scheduler"
	}

	if dest == "" {
		log.Fatal("--dest must point to a writable directory")
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		log.Fatalf("create dest directory: %v", err)
	}

	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, version, archiveName)
	tmpDir, err := os.MkdirTemp("", "p2-scheduler-action-*")
	if err != nil {
		log.Fatalf("create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, archiveName)
	if err := downloadFile(url, archivePath); err != nil {
		log.Fatalf("download archive: %v", err)
	}

	var binaryPath string
	if strings.HasSuffix(archiveName, ".tar.gz") {
		binaryPath, err = extractTarGz(archivePath, binaryName, tmpDir)
	} else {
		log.Fatal("zip extraction not implemented")
	}
	if err != nil {
		log.Fatalf("extract binary: %v", err)
	}

	finalPath := filepath.Join(dest, binaryName)
	if err := moveFile(binaryPath, finalPath); err != nil {
		log.Fatalf("move binary: %v", err)
	}

	if platformKey != "windows" {
		if err := os.Chmod(finalPath, 0o755); err != nil {
			log.Fatalf("chmod binary: %v", err)
		}
	}

	pathFile := os.Getenv("GITHUB_PATH")
	if pathFile == "" {
		log.Fatal("GITHUB_PATH is not set")
	}
	if err := appendLine(pathFile, dest); err != nil {
		log.Fatalf("update GITHUB_PATH: %v", err)
	}

	outputFile := os.Getenv("GITHUB_OUTPUT")
	if outputFile == "" {
		log.Fatal("GITHUB_OUTPUT is not set")
	}
	if err := appendLine(outputFile, fmt.Sprintf("binary=%s", finalPath)); err != nil {
		log.Fatalf("write GITHUB_OUTPUT: %v", err)
	}

	fmt.Printf("Installed p2-github-scheduler to %s\n", finalPath)
}

func titleCase(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func normalizeOS(osName string) (string, error) {
	switch strings.ToLower(osName) {
	case "linux":
		return "linux", nil
	case "macos", "darwin":
		return "darwin", nil
	case "windows":
		return "windows", nil
	default:
		return "", fmt.Errorf("unsupported operating system: %q", osName)
	}
}

func normalizeArch(arch string) (string, error) {
	switch strings.ToLower(arch) {
	case "amd64", "x86_64", "x64":
		return "amd64", nil
	case "arm64", "aarch64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported architecture: %q", arch)
	}
}

func archiveArch(arch string) string {
	switch arch {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "arm64"
	default:
		return arch
	}
}

func downloadFile(url, dest string) error {
	fmt.Printf("Downloading %s\n", url)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	return nil
}

func extractTarGz(archivePath, binaryName, destDir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		name := filepath.Base(header.Name)
		if name == binaryName {
			extracted := filepath.Join(destDir, binaryName)
			out, err := os.Create(extracted)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return "", err
			}
			out.Close()
			return extracted, nil
		}
	}
	return "", fmt.Errorf("%s not found in archive", binaryName)
}

func moveFile(src, dest string) error {
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	if err := copyFile(src, dest); err != nil {
		return err
	}
	return os.Remove(src)
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func appendLine(path, line string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := fmt.Fprintf(file, "%s\n", line); err != nil {
		return err
	}
	return nil
}
