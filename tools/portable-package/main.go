// SPDX-License-Identifier: MPL-2.0

package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type request struct {
	OS        string
	Arch      string
	Binary    string
	Output    string
	License   string
	Notices   string
	Icon      string
	InfoPlist string
}

type archiveFile struct {
	name string
	path string
}

func main() {
	var req request
	flag.StringVar(&req.OS, "os", "", "target operating system")
	flag.StringVar(&req.Arch, "arch", "", "target architecture label")
	flag.StringVar(&req.Binary, "binary", "", "built application binary")
	flag.StringVar(&req.Output, "output", "", "portable archive path")
	flag.StringVar(&req.License, "license", "", "repository license file")
	flag.StringVar(&req.Notices, "notices", "", "third-party notices file")
	flag.StringVar(&req.Icon, "icon", "", "macOS ICNS icon")
	flag.StringVar(&req.InfoPlist, "plist", "", "macOS Info.plist")
	flag.Parse()
	if err := packageArtifact(req); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func packageArtifact(req request) error {
	if req.Arch == "" || req.Binary == "" || req.Output == "" || req.License == "" || req.Notices == "" {
		return errors.New("arch, binary, output, license and notices are required")
	}
	if req.OS != "windows" && req.OS != "linux" && req.OS != "darwin" {
		return fmt.Errorf("unsupported target OS %q", req.OS)
	}
	if req.OS == "linux" && !strings.HasSuffix(req.Output, ".tar.gz") {
		return errors.New("Linux portable artifacts must use .tar.gz")
	}
	if req.OS != "linux" && !strings.HasSuffix(req.Output, ".zip") {
		return errors.New("Windows and macOS portable artifacts must use .zip")
	}

	files := []archiveFile{{name: "animeportable", path: req.Binary}}
	if req.OS == "windows" {
		files[0].name += ".exe"
	}
	files = append(files,
		archiveFile{name: "LICENSE", path: req.License},
		archiveFile{name: "THIRD_PARTY_NOTICES.md", path: req.Notices},
	)
	if req.OS == "darwin" {
		if req.Icon == "" || req.InfoPlist == "" {
			return errors.New("macOS packages require an ICNS icon and Info.plist")
		}
		files = []archiveFile{
			{name: "AnimePortable.app/Contents/MacOS/animeportable", path: req.Binary},
			{name: "AnimePortable.app/Contents/Resources/icons.icns", path: req.Icon},
			{name: "AnimePortable.app/Contents/Resources/LICENSE", path: req.License},
			{name: "AnimePortable.app/Contents/Resources/THIRD_PARTY_NOTICES.md", path: req.Notices},
			{name: "AnimePortable.app/Contents/Info.plist", path: req.InfoPlist},
		}
	}
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
	if req.OS == "linux" {
		err = writeTarGzip(output, files)
	} else {
		err = writeZip(output, files)
	}
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
	_, err = io.Copy(entry, input)
	return err
}

func writeTarGzip(output io.Writer, files []archiveFile) error {
	gzipWriter := gzip.NewWriter(output)
	gzipWriter.Header.ModTime = time.Time{}
	tarWriter := tar.NewWriter(gzipWriter)
	for _, file := range files {
		if err := addTarFile(tarWriter, file); err != nil {
			tarWriter.Close()
			gzipWriter.Close()
			return err
		}
	}
	if err := tarWriter.Close(); err != nil {
		gzipWriter.Close()
		return err
	}
	return gzipWriter.Close()
}

func addTarFile(writer *tar.Writer, file archiveFile) error {
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
	header := &tar.Header{Name: file.name, Mode: 0o755, Size: info.Size(), ModTime: time.Unix(0, 0).UTC(), Typeflag: tar.TypeReg, Format: tar.FormatUSTAR}
	if strings.HasSuffix(file.name, ".md") || filepath.Base(file.name) == "LICENSE" || strings.HasSuffix(file.name, ".plist") {
		header.Mode = 0o644
	}
	if err := writer.WriteHeader(header); err != nil {
		return err
	}
	_, err = io.Copy(writer, input)
	return err
}
