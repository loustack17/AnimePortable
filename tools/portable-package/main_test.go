// SPDX-License-Identifier: MPL-2.0

package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"animeportable/internal/runtimepin"
)

func TestCLIPackageRequiresEmbeddedRuntimePin(t *testing.T) {
	if err := validatePinnedRuntime(request{OS: "windows", Arch: "amd64"}); err == nil {
		t.Fatal("Windows package accepted without libmpv runtime")
	}
	if err := validatePinnedRuntime(request{OS: "windows", Arch: "amd64", LibMPV: "libmpv-2.dll", LibMPVSHA256: strings.Repeat("0", 64)}); err == nil {
		t.Fatal("different runtime digest accepted")
	}
	if err := validatePinnedRuntime(request{OS: "windows", Arch: "amd64", LibMPV: "libmpv-2.dll", LibMPVSHA256: strings.ToUpper(runtimepin.LibMPVSHA256)}); err != nil {
		t.Fatalf("matching runtime digest rejected: %v", err)
	}
	if err := validatePinnedRuntime(request{OS: "windows", Arch: "arm64", LibMPV: "libmpv-2.dll", LibMPVSHA256: runtimepin.LibMPVSHA256}); err == nil {
		t.Fatal("amd64 runtime accepted for another architecture")
	}
	if err := validatePinnedRuntime(request{}); err != nil {
		t.Fatalf("package without libmpv rejected: %v", err)
	}
}

func TestPortableArchivesContainOnlyExtractedAppPayload(t *testing.T) {
	root := t.TempDir()
	binary := writeFile(t, root, "animeportable.exe", "native executable")
	license := writeFile(t, root, "LICENSE", "license")
	notices := writeFile(t, root, "THIRD_PARTY_NOTICES.md", "notices")
	dll := writeFile(t, root, "libmpv-2.dll", "runtime")
	dllLicense := writeFile(t, root, "libmpv-license", "runtime license")
	digest := sha256.Sum256([]byte("runtime"))
	req := request{
		OS: "windows", Arch: "amd64", Binary: binary, Output: filepath.Join(root, "windows.zip"), License: license, Notices: notices,
		LibMPV: dll, LibMPVSHA256: hex.EncodeToString(digest[:]), LibMPVLicense: dllLicense,
	}
	if err := packageArtifact(req); err != nil {
		t.Fatal(err)
	}
	entries, contents, err := readZip(req.Output)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(entries)
	want := []string{"LICENSE", "THIRD_PARTY_NOTICES.md", "animeportable.exe", "libmpv-2.dll", "licenses/libmpv.txt"}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("entries = %v, want %v", entries, want)
	}
	for _, entry := range entries {
		for _, forbidden := range []string{"webview", "nsis", "installer", "frontend", "node_modules"} {
			if strings.Contains(strings.ToLower(entry), forbidden) {
				t.Fatalf("unexpected %s payload %q", forbidden, entry)
			}
		}
		if strings.HasPrefix(entry, "data/") {
			t.Fatalf("mutable portable data bundled inside archive: %s", entry)
		}
	}
	if contents["animeportable.exe"] == "" || contents["libmpv-2.dll"] == "" {
		t.Fatal("archive payload missing")
	}
}

func TestArchiveEntryNamesRejectTraversalAndPlatformSeparators(t *testing.T) {
	for _, name := range []string{"", "../outside", "AnimePortable.app/../../outside", "/absolute", "C:/outside", `folder\\file`, "folder//file", "./file"} {
		t.Run(strings.ReplaceAll(name, "/", "_"), func(t *testing.T) {
			if err := validateEntryName(name); err == nil {
				t.Fatalf("validateEntryName(%q) succeeded", name)
			}
		})
	}
	for _, name := range []string{"animeportable", "LICENSE", "AnimePortable.app/Contents/MacOS/animeportable"} {
		if err := validateEntryName(name); err != nil {
			t.Fatalf("validateEntryName(%q): %v", name, err)
		}
	}
}

func TestPortablePackageRejectsSymlinkInputsAndNeverOverwrites(t *testing.T) {
	root := t.TempDir()
	binary := writeFile(t, root, "binary", "executable")
	license := writeFile(t, root, "LICENSE", "original")
	notices := writeFile(t, root, "notices", "notices")
	link := filepath.Join(root, "linked-license")
	if err := os.Symlink(license, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	output := filepath.Join(root, "package.zip")
	dll := writeFile(t, root, "libmpv-2.dll", "runtime")
	dllLicense := writeFile(t, root, "libmpv-license", "runtime license")
	digest := sha256.Sum256([]byte("runtime"))
	req := request{OS: "windows", Arch: "amd64", Binary: binary, Output: output, License: link, Notices: notices, LibMPV: dll, LibMPVSHA256: hex.EncodeToString(digest[:]), LibMPVLicense: dllLicense}
	if err := packageArtifact(req); err == nil {
		t.Fatal("packageArtifact accepted symlink input")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("failed package left output: %v", err)
	}
	req.License = license
	if err := packageArtifact(req); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := packageArtifact(req); err == nil {
		t.Fatal("packageArtifact overwrote existing output")
	}
	contents, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "keep" {
		t.Fatalf("existing output changed: %q", contents)
	}
}

func TestPackageRequiresWindowsZipAndRejectsPausedPlatforms(t *testing.T) {
	root := t.TempDir()
	binary := writeFile(t, root, "binary", "binary")
	license := writeFile(t, root, "license", "license")
	notices := writeFile(t, root, "notices", "notices")
	base := request{OS: "windows", Arch: "amd64", Binary: binary, Output: filepath.Join(root, "package.msi"), License: license, Notices: notices}
	if err := packageArtifact(base); err == nil {
		t.Fatal("windows installer format accepted")
	}
	for _, target := range []string{"darwin", "linux"} {
		base.OS = target
		base.Output = filepath.Join(root, target+".zip")
		if err := packageArtifact(base); err == nil {
			t.Fatalf("paused %s platform accepted", target)
		}
	}
}

func TestWindowsPackagePinsBundledLibMPVAndIncludesLicense(t *testing.T) {
	root := t.TempDir()
	binary := writeFile(t, root, "binary", "application")
	license := writeFile(t, root, "license", "application license")
	notices := writeFile(t, root, "notices", "third-party notices")
	dll := writeFile(t, root, "libmpv-2.dll", "libmpv runtime")
	dllLicense := writeFile(t, root, "libmpv-license", "libmpv license")
	digest := sha256.Sum256([]byte("libmpv runtime"))
	req := request{
		OS: "windows", Arch: "amd64", Binary: binary, Output: filepath.Join(root, "portable.zip"),
		License: license, Notices: notices, LibMPV: dll, LibMPVSHA256: hex.EncodeToString(digest[:]), LibMPVLicense: dllLicense,
	}
	if err := packageArtifact(req); err != nil {
		t.Fatal(err)
	}
	entries, contents, err := readZip(req.Output)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(entries)
	want := []string{"LICENSE", "THIRD_PARTY_NOTICES.md", "animeportable.exe", "libmpv-2.dll", "licenses/libmpv.txt"}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("entries = %v, want %v", entries, want)
	}
	if contents["libmpv-2.dll"] != "libmpv runtime" || contents["licenses/libmpv.txt"] != "libmpv license" {
		t.Fatal("bundled runtime or license changed")
	}
	req.Output = filepath.Join(root, "bad.zip")
	req.LibMPVSHA256 = strings.Repeat("0", 64)
	if err := packageArtifact(req); err == nil {
		t.Fatal("incorrect runtime digest accepted")
	}
	if _, err := os.Stat(req.Output); !os.IsNotExist(err) {
		t.Fatalf("failed package left archive: %v", err)
	}
	req.LibMPVSHA256 = hex.EncodeToString(digest[:])
	req.LibMPVLicense = ""
	if err := packageArtifact(req); err == nil {
		t.Fatal("runtime without license accepted")
	}
	req.LibMPVLicense = dllLicense
	req.OS = "linux"
	req.Output = filepath.Join(root, "bad.tar.gz")
	if err := packageArtifact(req); err == nil {
		t.Fatal("Windows runtime accepted in Linux package")
	}
}

func writeFile(t *testing.T, root, name, contents string) string {
	t.Helper()
	file := filepath.Join(root, name)
	if err := os.WriteFile(file, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

func readZip(archive string) ([]string, map[string]string, error) {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return nil, nil, err
	}
	defer reader.Close()
	entries := make([]string, 0, len(reader.File))
	contents := make(map[string]string, len(reader.File))
	for _, file := range reader.File {
		entries = append(entries, file.Name)
		stream, err := file.Open()
		if err != nil {
			return nil, nil, err
		}
		data, readErr := io.ReadAll(stream)
		closeErr := stream.Close()
		if readErr != nil {
			return nil, nil, readErr
		}
		if closeErr != nil {
			return nil, nil, closeErr
		}
		contents[file.Name] = string(data)
	}
	return entries, contents, nil
}
