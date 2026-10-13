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
)

func TestPackageRequiresLibMPVMatchingExecutablePin(t *testing.T) {
	root := t.TempDir()
	req := communityFixture(t, root)

	req.Output = filepath.Join(root, "unpinned.zip")
	if err := packageArtifact(req, strings.Repeat("0", 64)); err == nil || !strings.Contains(err.Error(), "differs from the executable pin") {
		t.Fatalf("digest differing from pin: err = %v", err)
	}
	if _, err := os.Stat(req.Output); !os.IsNotExist(err) {
		t.Fatalf("rejected package left archive: %v", err)
	}

	noRuntime := req
	noRuntime.Output = filepath.Join(root, "no-runtime.zip")
	noRuntime.LibMPV = ""
	if err := packageArtifact(noRuntime, runtimeDigest); err == nil {
		t.Fatal("package without libmpv accepted")
	}

	upper := req
	upper.Output = filepath.Join(root, "upper.zip")
	upper.LibMPVSHA256 = strings.ToUpper(runtimeDigest)
	upper.LibMPVLicense = writeFile(t, root, "upper-notice.txt", communityNotice(upper.LibMPVSHA256))
	if err := packageArtifact(upper, runtimeDigest); err != nil {
		t.Fatalf("matching upper-case digest rejected: %v", err)
	}

	arm := req
	arm.Output = filepath.Join(root, "arm.zip")
	arm.Arch = "arm64"
	if err := packageArtifact(arm, runtimeDigest); err == nil {
		t.Fatal("amd64 runtime accepted for another architecture")
	}
}

func TestPortableArchivesContainOnlyExtractedAppPayload(t *testing.T) {
	root := t.TempDir()
	req := communityFixture(t, root)
	req.Output = filepath.Join(root, "windows.zip")
	if err := packageArtifact(req, runtimeDigest); err != nil {
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
	license := writeFile(t, root, "LICENSE", "original")
	link := filepath.Join(root, "linked-license")
	if err := os.Symlink(license, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	output := filepath.Join(root, "package.zip")
	req := communityFixture(t, root)
	req.Output = output
	req.License = link
	if err := packageArtifact(req, runtimeDigest); err == nil {
		t.Fatal("packageArtifact accepted symlink input")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("failed package left output: %v", err)
	}
	req.License = license
	if err := packageArtifact(req, runtimeDigest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := packageArtifact(req, runtimeDigest); err == nil {
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
	if err := packageArtifact(base, runtimeDigest); err == nil {
		t.Fatal("windows installer format accepted")
	}
	for _, target := range []string{"darwin", "linux"} {
		base.OS = target
		base.Output = filepath.Join(root, target+".zip")
		if err := packageArtifact(base, runtimeDigest); err == nil {
			t.Fatalf("paused %s platform accepted", target)
		}
	}
}

func TestWindowsPackagePinsBundledLibMPVAndIncludesLicense(t *testing.T) {
	root := t.TempDir()
	req := communityFixture(t, root)
	req.Output = filepath.Join(root, "portable.zip")
	dllLicense := req.LibMPVLicense
	runtimeHash := req.LibMPVSHA256
	if err := packageArtifact(req, runtimeDigest); err != nil {
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
	if contents["libmpv-2.dll"] != "runtime" || contents["licenses/libmpv.txt"] != communityNotice(runtimeHash) {
		t.Fatal("bundled runtime or license changed")
	}
	assertWrongDLLDigestRejected(t, root, req)
	req.LibMPVLicense = ""
	if err := packageArtifact(req, runtimeDigest); err == nil {
		t.Fatal("runtime without license accepted")
	}
	req.LibMPVLicense = dllLicense
	req.OS = "linux"
	req.Output = filepath.Join(root, "bad.tar.gz")
	if err := packageArtifact(req, runtimeDigest); err == nil {
		t.Fatal("Windows runtime accepted in Linux package")
	}
}

func TestCommunityRuntimePackageDisclosesUnverifiedSourceAndPinsDLL(t *testing.T) {
	root := t.TempDir()
	req := communityFixture(t, root)
	runtimeHash := req.LibMPVSHA256
	req.Output = filepath.Join(root, "community.zip")
	if err := packageArtifact(req, runtimeDigest); err != nil {
		t.Fatal(err)
	}
	entries, contents, err := readZip(req.Output)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(entries)
	want := []string{"LICENSE", "THIRD_PARTY_NOTICES.md", "animeportable.exe", "libmpv-2.dll", "licenses/libmpv.txt"}
	if !reflect.DeepEqual(entries, want) || contents["libmpv-2.dll"] != "runtime" {
		t.Fatalf("community entries = %v, want %v", entries, want)
	}
	req.Output = filepath.Join(root, "missing-disclosure.zip")
	req.LibMPVLicense = writeFile(t, root, "bad-notice.txt", "https://example.org/upstream\n")
	if err := packageArtifact(req, runtimeDigest); err == nil {
		t.Fatal("community package accepted notice without matching runtime hash and disclosure")
	}
	req.LibMPVLicense = writeFile(t, root, "valid-notice.txt", communityNotice(runtimeHash))
	assertWrongDLLDigestRejected(t, root, req)
}

// assertWrongDLLDigestRejected declares a digest the DLL does not have, with a
// matching notice and pin, so only the DLL byte check can fail.
func assertWrongDLLDigestRejected(t *testing.T, root string, req request) {
	t.Helper()
	wrong := strings.Repeat("0", 64)
	req.Output = filepath.Join(root, "wrong-dll.zip")
	req.LibMPVSHA256 = wrong
	req.LibMPVLicense = writeFile(t, root, "wrong-dll-notice.txt", communityNotice(wrong))
	err := packageArtifact(req, wrong)
	if err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch for libmpv-2.dll") {
		t.Fatalf("mismatched DLL digest: err = %v", err)
	}
	if _, err := os.Stat(req.Output); !os.IsNotExist(err) {
		t.Fatalf("failed package left archive: %v", err)
	}
}

// runtimeDigest is the SHA-256 of the fixture libmpv DLL contents.
var runtimeDigest = func() string {
	digest := sha256.Sum256([]byte("runtime"))
	return hex.EncodeToString(digest[:])
}()

func communityFixture(t *testing.T, root string) request {
	t.Helper()
	return request{
		OS: "windows", Arch: "amd64",
		Binary:        writeFile(t, root, "animeportable.exe", "application"),
		Output:        filepath.Join(root, "valid.zip"),
		License:       writeFile(t, root, "LICENSE", "application license"),
		Notices:       writeFile(t, root, "THIRD_PARTY_NOTICES.md", "third-party notices"),
		LibMPV:        writeFile(t, root, "libmpv-2.dll", "runtime"),
		LibMPVSHA256:  runtimeDigest,
		LibMPVLicense: writeFile(t, root, "libmpv-notice.txt", communityNotice(runtimeDigest)),
	}
}

func communityNotice(runtimeHash string) string {
	return runtimeHash + "\nhttps://example.org/upstream\nsource unverified\n"
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
