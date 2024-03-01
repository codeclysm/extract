package extract_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/codeclysm/extract/v3"
	"github.com/stretchr/testify/require"
)

func TestExtractors(t *testing.T) {
	type archiveTest struct {
		name string
		file *paths.Path
	}
	testCases := []archiveTest{
		{"TarGz", paths.New("testdata/archive.tar.gz")},
		{"TarBz2", paths.New("testdata/archive.tar.bz2")},
		{"TarXz", paths.New("testdata/archive.tar.xz")},
		{"TarZstd", paths.New("testdata/archive.tar.zst")},
		{"Zip", paths.New("testdata/archive.zip")},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			testArchive(t, test.file)
		})
	}
}

func testArchive(t *testing.T, archivePath *paths.Path) {
	tmp, err := paths.MkTempDir("", "")
	require.NoError(t, err)
	defer tmp.RemoveAll()

	data, err := archivePath.ReadFile()
	require.NoError(t, err)

	buffer := bytes.NewBuffer(data)

	extractor := extract.Extractor{
		FS: MockDisk{
			Base: tmp.String(),
		},
	}
	err = extractor.Archive(context.Background(), buffer, "/", nil)
	require.NoError(t, err)

	files := Files{
		"":                          "dir",
		"/archive":                  "dir",
		"/archive/folder":           "dir",
		"/archive/folderlink":       "link",
		"/archive/folder/file1.txt": "folder/File1",
		"/archive/file1.txt":        "File1",
		"/archive/file2.txt":        "File2",
		"/archive/link.txt":         "File1",
	}
	testWalk(t, tmp.String(), files)
}

func TestZipSlipHardening(t *testing.T) {
	{
		logger := &LoggingFS{}
		extractor := extract.Extractor{FS: logger}
		data, err := os.Open("testdata/zipslip/evil.zip")
		require.NoError(t, err)
		require.NoError(t, extractor.Zip(context.Background(), data, "/tmp/test", nil))
		require.NoError(t, data.Close())
		fmt.Print(logger)
		require.Empty(t, logger.Journal)
	}
	{
		logger := &LoggingFS{}
		extractor := extract.Extractor{FS: logger}
		data, err := os.Open("testdata/zipslip/evil.tar")
		require.NoError(t, err)
		require.NoError(t, extractor.Tar(context.Background(), data, "/tmp/test", nil))
		require.NoError(t, data.Close())
		fmt.Print(logger)
		require.Empty(t, logger.Journal)
	}

	if runtime.GOOS == "windows" {
		logger := &LoggingFS{}
		extractor := extract.Extractor{FS: logger}
		data, err := os.Open("testdata/zipslip/evil-win.tar")
		require.NoError(t, err)
		require.NoError(t, extractor.Tar(context.Background(), data, "/tmp/test", nil))
		require.NoError(t, data.Close())
		fmt.Print(logger)
		require.Empty(t, logger.Journal)
	}
}

func TestUnixPermissions(t *testing.T) {
	// Disable user's umask to enable creation of files with any permission, restore it after the test
	userUmask := UnixUmaskZero()
	defer UnixUmask(userUmask)

	archiveFilenames := []string{
		"testdata/permissions.zip",
		"testdata/permissions.tar",
	}
	for _, archiveFilename := range archiveFilenames {
		tmp, err := paths.MkTempDir("", "")
		require.NoError(t, err)
		defer tmp.RemoveAll()

		f, err := paths.New(archiveFilename).Open()
		require.NoError(t, err)
		err = extract.Archive(context.Background(), f, tmp.String(), nil)
		require.NoError(t, err)

		filepath.Walk(tmp.String(), func(path string, info os.FileInfo, _ error) error {
			filename := filepath.Base(path)
			// Desired permissions indicated by part of the filenames inside the zip/tar files
			if strings.HasPrefix(filename, "dir") {
				desiredPermString := strings.Split(filename, "dir")[1]
				desiredPerms, _ := strconv.ParseUint(desiredPermString, 8, 32)
				require.Equal(t, os.ModeDir|os.FileMode(OsDirPerms(desiredPerms)), info.Mode())
			} else if strings.HasPrefix(filename, "file") {
				desiredPermString := strings.Split(filename, "file")[1]
				desiredPerms, _ := strconv.ParseUint(desiredPermString, 8, 32)
				require.Equal(t, os.FileMode(OsFilePerms(desiredPerms)), info.Mode())
			}
			return nil
		})
	}
}

func TestZipDirectoryPermissions(t *testing.T) {
	// Disable user's umask to enable creation of files with any permission, restore it after the test
	userUmask := UnixUmaskZero()
	defer UnixUmask(userUmask)

	// This arduino library has files before their containing directories in the zip,
	// so a good test case that these directory permissions are created correctly
	archive := paths.New("testdata/filesbeforedirectories.zip")
	err := download(t, "https://downloads.arduino.cc/libraries/github.com/arduino-libraries/LiquidCrystal-1.0.7.zip", archive)
	require.NoError(t, err)

	tmp, err := paths.MkTempDir("", "")
	require.NoError(t, err)
	defer tmp.RemoveAll()

	f, err := archive.Open()
	require.NoError(t, err)
	err = extract.Archive(context.Background(), f, tmp.String(), nil)
	require.NoError(t, err)

	filepath.Walk(tmp.String(), func(path string, info os.FileInfo, _ error) error {
		// Test files and directories (excluding the parent) match permissions from the zip file
		if path != tmp.String() {
			if info.IsDir() {
				require.Equal(t, os.ModeDir|os.FileMode(OsDirPerms(0755)), info.Mode())
			} else {
				require.Equal(t, os.FileMode(OsFilePerms(0644)), info.Mode())
			}
		}
		return nil
	})
}

// MockDisk is a disk that chroots to a directory
type MockDisk struct {
	Base string
}

func (m MockDisk) Link(oldname, newname string) error {
	oldname = filepath.Join(m.Base, oldname)
	newname = filepath.Join(m.Base, newname)
	return os.Link(oldname, newname)
}

func (m MockDisk) MkdirAll(path string, perm os.FileMode) error {
	path = filepath.Join(m.Base, path)
	return os.MkdirAll(path, perm)
}

func (m MockDisk) Symlink(oldname, newname string) error {
	oldname = filepath.Join(m.Base, oldname)
	newname = filepath.Join(m.Base, newname)
	return os.Symlink(oldname, newname)
}

func (m MockDisk) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	name = filepath.Join(m.Base, name)
	return os.OpenFile(name, flag, perm)
}

func (m MockDisk) Stat(name string) (os.FileInfo, error) {
	name = filepath.Join(m.Base, name)
	return os.Stat(name)
}

func (m MockDisk) Chmod(name string, mode os.FileMode) error {
	name = filepath.Join(m.Base, name)
	return os.Chmod(name, mode)
}
