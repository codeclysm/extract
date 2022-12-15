package extract_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
		{"TarXz", paths.New("testdata/archive.tar.xz")},
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
