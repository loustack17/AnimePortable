// SPDX-License-Identifier: MPL-2.0

package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	OS               string
	Arch             string
	Binary           string
	Output           string
	License          string
	Notices          string
	LibMPV           string
	LibMPVSHA256     string
	LibMPVLicense    string
	LibMPVManifest   string
	LibMPVSources    string
	FLTKSource       string
	FLTKSourceSHA256 string
	FLTKPatch        string
	FLTKPatchSHA256  string
}

type archiveFile struct {
	name string
	path string
	hash string
}

type provenanceManifest struct {
	Schema        int                   `json:"schema"`
	LibMPVSHA256  string                `json:"libmpv_sha256"`
	SourcesSHA256 string                `json:"sources_sha256"`
	Components    []provenanceComponent `json:"components"`
}

type provenanceComponent struct {
	Name        string `json:"name"`
	Revision    string `json:"revision"`
	License     string `json:"license"`
	SourceEntry string `json:"source_entry"`
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
	flag.StringVar(&req.LibMPVManifest, "libmpv-manifest", "", "JSON provenance manifest for bundled libmpv runtime")
	flag.StringVar(&req.LibMPVSources, "libmpv-sources", "", "ZIP archive of corresponding libmpv dependency sources")
	flag.StringVar(&req.FLTKSource, "fltk-source", "", "FLTK 1.4.5 source archive")
	flag.StringVar(&req.FLTKSourceSHA256, "fltk-source-sha256", "", "expected SHA-256 of FLTK source archive")
	flag.StringVar(&req.FLTKPatch, "fltk-patch", "", "go-fltk Windows source patch")
	flag.StringVar(&req.FLTKPatchSHA256, "fltk-patch-sha256", "", "expected SHA-256 of FLTK patch")
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
	if req.LibMPV == "" || req.LibMPVLicense == "" || req.LibMPVManifest == "" || req.LibMPVSources == "" {
		return errors.New("Windows package requires libmpv runtime, license notice, provenance manifest and corresponding sources")
	}
	if req.FLTKSource == "" || req.FLTKPatch == "" || !validSHA256(req.FLTKSourceSHA256) || !validSHA256(req.FLTKPatchSHA256) {
		return errors.New("Windows package requires FLTK source archive, Windows patch and their SHA-256 digests")
	}
	decoded, err := hex.DecodeString(req.LibMPVSHA256)
	if err != nil || len(decoded) != sha256.Size {
		return errors.New("libmpv runtime requires a valid SHA-256 digest")
	}
	manifest, err := validateProvenance(req)
	if err != nil {
		return err
	}

	files := []archiveFile{{name: "animeportable.exe", path: req.Binary}}
	files = append(files,
		archiveFile{name: "LICENSE", path: req.License},
		archiveFile{name: "THIRD_PARTY_NOTICES.md", path: req.Notices},
	)
	files = append(files,
		archiveFile{name: "libmpv-2.dll", path: req.LibMPV, hash: strings.ToLower(req.LibMPVSHA256)},
		archiveFile{name: "licenses/libmpv.txt", path: req.LibMPVLicense},
		archiveFile{name: "licenses/libmpv-provenance.json", path: req.LibMPVManifest, hash: manifest.manifestHash},
		archiveFile{name: "sources/libmpv-sources.zip", path: req.LibMPVSources, hash: manifest.sourcesHash},
		archiveFile{name: "sources/fltk-1.4.5.tar.gz", path: req.FLTKSource, hash: strings.ToLower(req.FLTKSourceSHA256)},
		archiveFile{name: "sources/fltk-1.4.patch", path: req.FLTKPatch, hash: strings.ToLower(req.FLTKPatchSHA256)},
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

type validatedProvenance struct {
	manifestHash string
	sourcesHash  string
}

func validateProvenance(req request) (validatedProvenance, error) {
	var result validatedProvenance
	manifestFile, err := openRegularFile(req.LibMPVManifest)
	if err != nil {
		return result, fmt.Errorf("invalid libmpv provenance manifest: %w", err)
	}
	defer manifestFile.Close()
	info, err := manifestFile.Stat()
	if err != nil {
		return result, err
	}
	if info.Size() > 1<<20 {
		return result, errors.New("libmpv provenance manifest exceeds 1 MiB")
	}
	manifestBytes, err := io.ReadAll(manifestFile)
	if err != nil {
		return result, err
	}
	var manifest provenanceManifest
	decoder := json.NewDecoder(strings.NewReader(string(manifestBytes)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return result, fmt.Errorf("decode libmpv provenance manifest: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return result, errors.New("libmpv provenance manifest must contain one JSON object")
	}
	if manifest.Schema != 1 {
		return result, fmt.Errorf("unsupported libmpv provenance schema %d", manifest.Schema)
	}
	if !strings.EqualFold(manifest.LibMPVSHA256, req.LibMPVSHA256) {
		return result, errors.New("provenance manifest libmpv digest does not match runtime")
	}
	if !validSHA256(manifest.SourcesSHA256) {
		return result, errors.New("provenance manifest requires a valid sources SHA-256 digest")
	}
	if len(manifest.Components) < 2 {
		return result, errors.New("provenance manifest requires mpv and FFmpeg components")
	}
	components := make(map[string]bool, len(manifest.Components))
	sourceEntries := make(map[string]bool, len(manifest.Components))
	for _, component := range manifest.Components {
		name := strings.ToLower(strings.TrimSpace(component.Name))
		if name == "" || strings.TrimSpace(component.Revision) == "" || strings.TrimSpace(component.License) == "" || component.SourceEntry == "" {
			return result, errors.New("each provenance component requires name, exact revision, license and source entry")
		}
		if err := validateEntryName(component.SourceEntry); err != nil {
			return result, fmt.Errorf("unsafe provenance source entry: %w", err)
		}
		if components[name] || sourceEntries[component.SourceEntry] {
			return result, errors.New("provenance component names and source entries must be unique")
		}
		components[name] = true
		sourceEntries[component.SourceEntry] = true
	}
	if !components["mpv"] || !components["ffmpeg"] {
		return result, errors.New("provenance manifest must include mpv and FFmpeg")
	}
	if err := validateSourceArchive(req.LibMPVSources, manifest.SourcesSHA256, sourceEntries); err != nil {
		return result, err
	}
	manifestDigest := sha256.Sum256(manifestBytes)
	result.manifestHash = hex.EncodeToString(manifestDigest[:])
	result.sourcesHash = strings.ToLower(manifest.SourcesSHA256)
	return result, nil
}

func validSHA256(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func validateSourceArchive(filePath, expectedHash string, required map[string]bool) error {
	input, err := openRegularFile(filePath)
	if err != nil {
		return fmt.Errorf("invalid libmpv source archive: %w", err)
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, input); err != nil {
		input.Close()
		return err
	}
	if err := input.Close(); err != nil {
		return err
	}
	if !strings.EqualFold(hex.EncodeToString(hasher.Sum(nil)), expectedHash) {
		return errors.New("libmpv source archive SHA-256 does not match provenance manifest")
	}
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return fmt.Errorf("open libmpv source archive: %w", err)
	}
	defer reader.Close()
	found := make(map[string]bool, len(required))
	for _, file := range reader.File {
		name := file.Name
		isDirectory := file.Mode().IsDir()
		if isDirectory {
			name = strings.TrimSuffix(name, "/")
		}
		if err := validateEntryName(name); err != nil {
			return fmt.Errorf("unsafe libmpv source entry: %w", err)
		}
		if file.Mode()&os.ModeSymlink != 0 || (!isDirectory && !file.Mode().IsRegular()) {
			return fmt.Errorf("libmpv source entry %q is not a regular file", file.Name)
		}
		if isDirectory {
			continue
		}
		if !required[file.Name] {
			continue
		}
		if found[file.Name] {
			return fmt.Errorf("duplicate libmpv source entry %q", file.Name)
		}
		found[file.Name] = true
		stream, err := file.Open()
		if err != nil {
			return err
		}
		size, readErr := io.Copy(io.Discard, stream)
		closeErr := stream.Close()
		if readErr != nil {
			return fmt.Errorf("read libmpv source entry %q: %w", file.Name, readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close libmpv source entry %q: %w", file.Name, closeErr)
		}
		if size == 0 {
			return fmt.Errorf("libmpv source entry %q is empty", file.Name)
		}
	}
	for name := range required {
		if !found[name] {
			return fmt.Errorf("libmpv source archive is missing required source entry %q", name)
		}
	}
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
		return fmt.Errorf("SHA-256 mismatch for %s", file.name)
	}
	return nil
}
