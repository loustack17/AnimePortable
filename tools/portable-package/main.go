// SPDX-License-Identifier: MPL-2.0

package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"animeportable/internal/runtimepin"
)

type request struct {
	OS            string
	Arch          string
	Binary        string
	Output        string
	License       string
	Notices       string
	LibMPV        string
	LibMPVSHA256  string
	LibMPVLicense string
}

type archiveFile struct {
	name string
	path string
	hash string
}

func main() {
	var req request
	flag.StringVar(&req.OS, "os", "", "target operating system")
	flag.StringVar(&req.Arch, "arch", "", "target architecture label")
	flag.StringVar(&req.Binary, "binary", "", "built application binary")
	flag.StringVar(&req.Output, "output", "", "portable archive path")
	flag.StringVar(&req.License, "license", "", "repository license file")
	flag.StringVar(&req.Notices, "notices", "", "third-party notices file")
	flag.StringVar(&req.LibMPV, "libmpv", "", "Windows libmpv runtime DLL")
	flag.StringVar(&req.LibMPVSHA256, "libmpv-sha256", "", "expected SHA-256 of libmpv runtime DLL")
	flag.StringVar(&req.LibMPVLicense, "libmpv-license", "", "license notice for bundled libmpv runtime")
	flag.Parse()
	if err := validatePinnedRuntime(req); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := packageArtifact(req); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func validatePinnedRuntime(req request) error {
	if req.OS == "windows" && req.LibMPV == "" {
		return errors.New("Windows portable package requires the pinned libmpv runtime")
	}
	if req.LibMPV != "" {
		if req.OS != "windows" || req.Arch != "amd64" {
			return errors.New("pinned libmpv runtime supports Windows amd64 only")
		}
		if !strings.EqualFold(req.LibMPVSHA256, runtimepin.LibMPVSHA256) {
			return errors.New("libmpv runtime digest differs from the executable pin")
		}
	}
	return nil
}

func packageArtifact(req request) error {
	if req.Arch == "" || req.Binary == "" || req.Output == "" || req.License == "" || req.Notices == "" {
		return errors.New("arch, binary, output, license and notices are required")
	}
	if req.OS != "windows" || req.Arch != "amd64" {
		return fmt.Errorf("unsupported target OS %q", req.OS)
	}
	if !strings.HasSuffix(req.Output, ".zip") {
		return errors.New("Windows portable artifacts must use .zip")
	}
	if req.LibMPV == "" || req.LibMPVLicense == "" {
		return errors.New("Windows package requires libmpv runtime and license notice")
	}
	decoded, err := hex.DecodeString(req.LibMPVSHA256)
	if err != nil || len(decoded) != sha256.Size {
		return errors.New("libmpv runtime requires a valid SHA-256 digest")
	}

	files := []archiveFile{{name: "animeportable.exe", path: req.Binary}}
	files = append(files,
		archiveFile{name: "LICENSE", path: req.License},
		archiveFile{name: "THIRD_PARTY_NOTICES.md", path: req.Notices},
	)
	files = append(files,
		archiveFile{name: "libmpv-2.dll", path: req.LibMPV, hash: strings.ToLower(req.LibMPVSHA256)},
		archiveFile{name: "licenses/libmpv.txt", path: req.LibMPVLicense},
	)
	for _, file := range files {
		if err := validateEntryName(file.name); err != nil {
			return err
		}
		if err := validateRegularFile(file.path); err != nil {
			return fmt.Errorf("invalid input %s: %w", file.path, err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(req.Output), 0o755); err != nil {
		return err
	}
	output, err := os.OpenFile(req.Output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create archive without overwriting existing files: %w", err)
	}
	completed := false
	defer func() {
		output.Close()
		if !completed {
			os.Remove(req.Output)
		}
	}()
	err = writeZip(output, files)
	if err != nil {
		return err
	}
	if err := output.Sync(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	completed = true
	return nil
}

func validateEntryName(name string) error {
	if name == "" || strings.ContainsAny(name, "\\:") || strings.HasPrefix(name, "/") || path.Clean(name) != name {
		return fmt.Errorf("unsafe archive entry name %q", name)
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("unsafe archive entry name %q", name)
		}
	}
	return nil
}

func validateRegularFile(filePath string) error {
	input, err := openRegularFile(filePath)
	if err != nil {
		return err
	}
	return input.Close()
}

func openRegularFile(filePath string) (*os.File, error) {
	before, err := os.Lstat(filePath)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, errors.New("expected a regular file")
	}
	input, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	after, err := input.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		input.Close()
		if err != nil {
			return nil, err
		}
		return nil, errors.New("input changed while opening")
	}
	return input, nil
}

func writeZip(output io.Writer, files []archiveFile) error {
	writer := zip.NewWriter(output)
	for _, file := range files {
		if err := addZipFile(writer, file); err != nil {
			writer.Close()
			return err
		}
	}
	return writer.Close()
}

func addZipFile(writer *zip.Writer, file archiveFile) error {
	if err := validateEntryName(file.name); err != nil {
		return err
	}
	input, err := openRegularFile(file.path)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = file.name
	header.Method = zip.Deflate
	header.Modified = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	if file.hash == "" {
		_, err = io.Copy(entry, input)
		return err
	}
	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(entry, hasher), input); err != nil {
		return err
	}
	if hex.EncodeToString(hasher.Sum(nil)) != file.hash {
		return errors.New("libmpv runtime SHA-256 mismatch")
	}
	return nil
}
