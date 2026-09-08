// Command dist builds signed-off release archives and a checksum manifest.
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type target struct {
	os   string
	arch string
}

var targets = []target{
	{os: "darwin", arch: "amd64"},
	{os: "darwin", arch: "arm64"},
	{os: "linux", arch: "amd64"},
	{os: "linux", arch: "arm64"},
	{os: "windows", arch: "amd64"},
	{os: "windows", arch: "arm64"},
}

func main() {
	version := flag.String("version", "", "release version, such as v0.1.0-beta.1")
	outDir := flag.String("out", "dist", "output directory")
	flag.Parse()
	if !validVersion(*version) {
		fatalf("invalid -version %q (expected vMAJOR.MINOR.PATCH with optional prerelease)", *version)
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatalf("create output: %v", err)
	}
	displayVersion := strings.TrimPrefix(*version, "v")
	for _, item := range targets {
		if err := buildTarget(item, displayVersion, *outDir); err != nil {
			fatalf("%s/%s: %v", item.os, item.arch, err)
		}
	}
	if err := writeChecksums(*outDir); err != nil {
		fatalf("checksums: %v", err)
	}
}

func validVersion(version string) bool {
	if !strings.HasPrefix(version, "v") {
		return false
	}
	base := strings.TrimPrefix(strings.SplitN(version, "-", 2)[0], "v")
	parts := strings.Split(base, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, char := range part {
			if char < '0' || char > '9' {
				return false
			}
		}
	}
	return true
}

func buildTarget(item target, version, outDir string) error {
	name := "ctxt"
	if item.os == "windows" {
		name += ".exe"
	}
	tempDir, err := os.MkdirTemp("", "ctxt-dist-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	binary := filepath.Join(tempDir, name)
	ldflags := fmt.Sprintf("-s -w -X github.com/ktappdev/contexting.Version=%s", version)
	command := exec.Command("go", "build", "-trimpath", "-ldflags", ldflags, "-o", binary, "./cmd/ctxt")
	command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+item.os, "GOARCH="+item.arch)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("build: %w: %s", err, strings.TrimSpace(string(output)))
	}
	base := fmt.Sprintf("ctxt_%s_%s_%s", version, item.os, item.arch)
	files := []archiveFile{
		{name: name, path: binary, mode: 0o755},
		{name: "LICENSE", path: "LICENSE", mode: 0o644},
		{name: "README.md", path: "README.md", mode: 0o644},
	}
	if item.os == "windows" {
		return writeZip(filepath.Join(outDir, base+".zip"), files)
	}
	return writeTarGz(filepath.Join(outDir, base+".tar.gz"), files)
}

type archiveFile struct {
	name string
	path string
	mode int64
}

func writeTarGz(path string, files []archiveFile) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gz)
	for _, item := range files {
		info, err := os.Stat(item.path)
		if err != nil {
			return closeTar(file, gz, tarWriter, err)
		}
		header := &tar.Header{Name: item.name, Mode: item.mode, Size: info.Size()}
		if err := tarWriter.WriteHeader(header); err != nil {
			return closeTar(file, gz, tarWriter, err)
		}
		input, err := os.Open(item.path)
		if err != nil {
			return closeTar(file, gz, tarWriter, err)
		}
		_, copyErr := io.Copy(tarWriter, input)
		closeErr := input.Close()
		if copyErr != nil {
			return closeTar(file, gz, tarWriter, copyErr)
		}
		if closeErr != nil {
			return closeTar(file, gz, tarWriter, closeErr)
		}
	}
	return closeTar(file, gz, tarWriter, nil)
}

func closeTar(file *os.File, gz *gzip.Writer, tarWriter *tar.Writer, prior error) error {
	for _, closer := range []io.Closer{tarWriter, gz, file} {
		if err := closer.Close(); prior == nil && err != nil {
			prior = err
		}
	}
	return prior
}

func writeZip(path string, files []archiveFile) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(file)
	for _, item := range files {
		info, err := os.Stat(item.path)
		if err != nil {
			return closeZip(file, writer, err)
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return closeZip(file, writer, err)
		}
		header.Name = item.name
		header.Method = zip.Deflate
		header.SetMode(os.FileMode(item.mode))
		destination, err := writer.CreateHeader(header)
		if err != nil {
			return closeZip(file, writer, err)
		}
		input, err := os.Open(item.path)
		if err != nil {
			return closeZip(file, writer, err)
		}
		_, copyErr := io.Copy(destination, input)
		closeErr := input.Close()
		if copyErr != nil {
			return closeZip(file, writer, copyErr)
		}
		if closeErr != nil {
			return closeZip(file, writer, closeErr)
		}
	}
	return closeZip(file, writer, nil)
}

func closeZip(file *os.File, writer *zip.Writer, prior error) error {
	if err := writer.Close(); prior == nil && err != nil {
		prior = err
	}
	if err := file.Close(); prior == nil && err != nil {
		prior = err
	}
	return prior
}

func writeChecksums(outDir string) error {
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return err
	}
	var lines []string
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "checksums.txt" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(outDir, entry.Name()))
		if err != nil {
			return err
		}
		lines = append(lines, fmt.Sprintf("%x  %s", sha256.Sum256(data), entry.Name()))
	}
	sort.Strings(lines)
	return os.WriteFile(filepath.Join(outDir, "checksums.txt"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "dist: "+format+"\n", args...)
	os.Exit(1)
}
