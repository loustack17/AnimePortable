//go:build windows && amd64

// SPDX-License-Identifier: MPL-2.0

package mpvwin

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

var (
	ErrInvalidLibrary = errors.New("libmpv: invalid library")
	ErrLibraryHash    = errors.New("libmpv: library hash mismatch")
	ErrLibraryLoad    = errors.New("libmpv: library load failed")
	ErrMissingSymbol  = errors.New("libmpv: required entry point unavailable")
	ErrLibraryClosed  = errors.New("libmpv: library closed")
	ErrLibraryInUse   = errors.New("libmpv: library still in use")
	ErrPlayerFailed   = errors.New("libmpv: player operation failed")
	ErrPlayerClosed   = errors.New("libmpv: player closed")
)

const (
	loadLibrarySearchDLLLoadDir = 0x00000100
	loadLibrarySearchSystem32   = 0x00000800
)

type moduleLoader interface {
	Load(string) (uintptr, error)
	Symbol(uintptr, string) (uintptr, error)
	Free(uintptr) error
}

type windowsLoader struct{}

func (windowsLoader) Load(path string) (uintptr, error) {
	handle, err := windows.LoadLibraryEx(path, 0, loadLibrarySearchDLLLoadDir|loadLibrarySearchSystem32)
	if err != nil {
		return 0, ErrLibraryLoad
	}
	return uintptr(handle), nil
}

func (windowsLoader) Symbol(handle uintptr, name string) (uintptr, error) {
	address, err := windows.GetProcAddress(windows.Handle(handle), name)
	if err != nil {
		return 0, ErrMissingSymbol
	}
	return address, nil
}

func (windowsLoader) Free(handle uintptr) error {
	if err := windows.FreeLibrary(windows.Handle(handle)); err != nil {
		return ErrLibraryLoad
	}
	return nil
}

func validateLibraryFile(path, expectedSHA256 string) (string, *os.File, error) {
	if !filepath.IsAbs(path) || !validSHA256(expectedSHA256) {
		return "", nil, ErrInvalidLibrary
	}
	path = filepath.Clean(path)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", nil, ErrInvalidLibrary
	}
	widePath, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", nil, ErrInvalidLibrary
	}
	attributes, err := windows.GetFileAttributes(widePath)
	if err != nil || attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return "", nil, ErrInvalidLibrary
	}
	handle, err := windows.CreateFile(widePath, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return "", nil, ErrInvalidLibrary
	}
	var heldInfo windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(handle, &heldInfo) != nil || heldInfo.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = windows.CloseHandle(handle)
		return "", nil, ErrInvalidLibrary
	}
	file := os.NewFile(uintptr(handle), path)
	resolved, err := finalLibraryPath(handle)
	if err != nil {
		_ = file.Close()
		return "", nil, ErrInvalidLibrary
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	if copyErr != nil || !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), expectedSHA256) {
		_ = file.Close()
		return "", nil, ErrLibraryHash
	}
	return resolved, file, nil
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func finalLibraryPath(handle windows.Handle) (string, error) {
	buffer := make([]uint16, 512)
	for len(buffer) <= 32768 {
		length, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), 0)
		if err != nil || length == 0 {
			return "", ErrInvalidLibrary
		}
		if int(length) < len(buffer) {
			return windows.UTF16ToString(buffer[:length]), nil
		}
		buffer = make([]uint16, length+1)
	}
	return "", ErrInvalidLibrary
}

func openLibrary(path, expectedSHA256 string, loader moduleLoader) (*Library, error) {
	if loader == nil {
		return nil, ErrInvalidLibrary
	}
	path, file, err := validateLibraryFile(path, expectedSHA256)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	handle, err := loader.Load(path)
	if err != nil || handle == 0 {
		return nil, ErrLibraryLoad
	}
	api, err := bindAPI(handle, loader.Symbol)
	if err != nil {
		_ = loader.Free(handle)
		return nil, err
	}
	return &Library{handle: handle, api: api, loader: loader}, nil
}
