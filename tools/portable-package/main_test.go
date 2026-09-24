// SPDX-License-Identifier: MPL-2.0

package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestPortableArchivesContainOnlyExtractedAppPayload(t *testing.T) {
	root := t.TempDir()
	binary := writeFile(t, root, "animeportable.exe", "native executable")
	license := writeFile(t, root, "LICENSE", "license")
	notices := writeFile(t, root, "THIRD_PARTY_NOTICES.md", "notices")
	icon := writeFile(t, root, "icons.icns", "icon")
	plist := writeFile(t, root, "Info.plist", "plist")

	cases := []struct {
		name    string
		os      string
		output  string
		want    []string
		archive func(string) ([]string, map[string]string, error)
	}{
		{
			name:    "windows root extraction",
			os:      "windows",
			output:  filepath.Join(root, "windows.zip"),
			want:    []string{"LICENSE", "THIRD_PARTY_NOTICES.md", "animeportable.exe"},
			archive: readZip,
		},
		{
			name:    "linux root extraction",
			os:      "linux",
			output:  filepath.Join(root, "linux.tar.gz"),
			want:    []string{"LICENSE", "THIRD_PARTY_NOTICES.md", "animeportable"},
			archive: readTarGzip,
		},
		{
			name:   "mac app and sibling data root",
			os:     "darwin",
			output: filepath.Join(root, "mac.zip"),
			want: []string{
				"AnimePortable.app/Contents/Info.plist",
				"AnimePortable.app/Contents/MacOS/animeportable",
				"AnimePortable.app/Contents/Resources/LICENSE",
				"AnimePortable.app/Contents/Resources/THIRD_PARTY_NOTICES.md",
				"AnimePortable.app/Contents/Resources/icons.icns",
			},
			archive: readZip,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := request{OS: tc.os, Arch: "amd64", Binary: binary, Output: tc.output, License: license, Notices: notices}
			if tc.os == "darwin" {
				req.Icon = icon
				req.InfoPlist = plist
			}
			if err := packageArtifact(req); err != nil {
				t.Fatal(err)
			}
			entries, contents, err := tc.archive(tc.output)
			if err != nil {
				t.Fatal(err)
			}
			sort.Strings(entries)
			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			if !reflect.DeepEqual(entries, want) {
				t.Fatalf("entries = %v, want %v", entries, want)
			}
			for _, forbidden := range []string{"mpv", "webview", "nsis", "installer", "frontend", "node_modules"} {
				for _, entry := range entries {
					if strings.Contains(strings.ToLower(entry), forbidden) {
						t.Fatalf("unexpected %s payload %q", forbidden, entry)
					}
				}
			}
			if strings.Contains(tc.os, "darwin") {
				if !strings.HasPrefix(entries[0], "AnimePortable.app/") {
					t.Fatalf("macOS app is not at extraction root: %v", entries)
				}
				for _, entry := range entries {
					if strings.Contains(entry, "/data/") || strings.HasPrefix(entry, "data/") {
						t.Fatalf("mutable portable data bundled inside archive: %s", entry)
					}
				}
			}
			if contents[tc.want[0]] == "" {
				t.Fatalf("archive content missing for %s", tc.want[0])
			}
		})
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
	req := request{OS: "windows", Arch: "amd64", Binary: binary, Output: output, License: link, Notices: notices}
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

func TestPackageRequiresPortableFormatsAndMacBundleAssets(t *testing.T) {
	root := t.TempDir()
	binary := writeFile(t, root, "binary", "binary")
	license := writeFile(t, root, "license", "license")
	notices := writeFile(t, root, "notices", "notices")
	base := request{OS: "windows", Arch: "amd64", Binary: binary, Output: filepath.Join(root, "package.msi"), License: license, Notices: notices}
	if err := packageArtifact(base); err == nil {
		t.Fatal("windows installer format accepted")
	}
	base.OS = "darwin"
	base.Output = filepath.Join(root, "mac.zip")
	if err := packageArtifact(base); err == nil {
		t.Fatal("macOS package without bundle icon and plist accepted")
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

func readTarGzip(archive string) ([]string, map[string]string, error) {
	file, err := os.Open(archive)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, nil, err
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	entries := []string{}
	contents := map[string]string{}
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		entries = append(entries, header.Name)
		data, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, nil, err
		}
		contents[header.Name] = string(data)
	}
	return entries, contents, nil
}
