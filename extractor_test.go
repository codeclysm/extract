package extract_test

import (
	"archive/tar"
	"archive/zip"
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
	"github.com/codeclysm/extract/v4"
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
	t.Run("ZipTraversal", func(t *testing.T) {
		logger := &LoggingFS{}
		extractor := extract.Extractor{FS: logger}
		data, err := os.Open("testdata/zipslip/evil.zip")
		require.NoError(t, err)
		require.NoError(t, extractor.Zip(context.Background(), data, "/tmp/test", nil))
		require.NoError(t, data.Close())
		fmt.Print(logger)
		require.Empty(t, logger.Journal)
	})

	t.Run("TarTraversal", func(t *testing.T) {
		logger := &LoggingFS{}
		extractor := extract.Extractor{FS: logger}
		data, err := os.Open("testdata/zipslip/evil.tar")
		require.NoError(t, err)
		require.NoError(t, extractor.Tar(context.Background(), data, "/tmp/test", nil))
		require.NoError(t, data.Close())
		fmt.Print(logger)
		require.Empty(t, logger.Journal)
	})

	t.Run("TarLinkTraversal", func(t *testing.T) {
		logger := &LoggingFS{}
		extractor := extract.Extractor{FS: logger}
		data, err := os.Open("testdata/zipslip/evil-link-traversal.tar")
		require.NoError(t, err)
		require.NoError(t, extractor.Tar(context.Background(), data, "/tmp/test", nil))
		require.NoError(t, data.Close())
		fmt.Print(logger)
		require.Empty(t, logger.Journal)
	})

	t.Run("WindowsTarTraversal", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("Skipped on non-Windows host")
		}
		logger := &LoggingFS{}
		extractor := extract.Extractor{FS: logger}
		data, err := os.Open("testdata/zipslip/evil-win.tar")
		require.NoError(t, err)
		require.NoError(t, extractor.Tar(context.Background(), data, "/tmp/test", nil))
		require.NoError(t, data.Close())
		fmt.Print(logger)
		require.Empty(t, logger.Journal)
	})
}

func mkTempDir(t *testing.T) *paths.Path {
	tmp, err := paths.MkTempDir("", "test")
	require.NoError(t, err)
	t.Cleanup(func() { tmp.RemoveAll() })
	return tmp
}

func TestSymLinkMazeHardening(t *testing.T) {
	addTarSymlink := func(t *testing.T, tw *tar.Writer, new, old string) {
		err := tw.WriteHeader(&tar.Header{
			Mode: 0o0777, Typeflag: tar.TypeSymlink, Name: new, Linkname: old,
		})
		require.NoError(t, err)
	}
	addZipSymlink := func(t *testing.T, zw *zip.Writer, new, old string) {
		h := &zip.FileHeader{Name: new, Method: zip.Deflate}
		h.SetMode(os.ModeSymlink)
		w, err := zw.CreateHeader(h)
		require.NoError(t, err)
		_, err = w.Write([]byte(old))
		require.NoError(t, err)
	}

	t.Run("TarWithSymlinkToAbsPath", func(t *testing.T) {
		// Create target dir
		tmp := mkTempDir(t)
		targetDir := tmp.Join("test")
		require.NoError(t, targetDir.Mkdir())

		// Make a tar archive with symlink maze
		outputTar := bytes.NewBuffer(nil)
		tw := tar.NewWriter(outputTar)
		addTarSymlink(t, tw, "aaa", tmp.String())
		addTarSymlink(t, tw, "aaa/sym", "something")
		require.NoError(t, tw.Close())

		// Run extract
		extractor := extract.Extractor{FS: &LoggingFS{}}
		require.Error(t, extractor.Tar(context.Background(), outputTar, targetDir.String(), nil))
		require.NoFileExists(t, tmp.Join("sym").String())
	})

	t.Run("ZipWithSymlinkToAbsPath", func(t *testing.T) {
		// Create target dir
		tmp := mkTempDir(t)
		targetDir := tmp.Join("test")
		require.NoError(t, targetDir.Mkdir())

		// Make a zip archive with symlink maze
		outputZip := bytes.NewBuffer(nil)
		zw := zip.NewWriter(outputZip)
		addZipSymlink(t, zw, "aaa", tmp.String())
		addZipSymlink(t, zw, "aaa/sym", "something")
		require.NoError(t, zw.Close())

		// Run extract
		extractor := extract.Extractor{FS: &LoggingFS{}}
		err := extractor.Zip(context.Background(), outputZip, targetDir.String(), nil)
		require.NoFileExists(t, tmp.Join("sym").String())
		require.Error(t, err)
	})

	t.Run("TarWithSymlinkToRelativeExternalPath", func(t *testing.T) {
		// Create target dir
		tmp := mkTempDir(t)
		targetDir := tmp.Join("test")
		require.NoError(t, targetDir.Mkdir())
		checkDir := tmp.Join("secret")
		require.NoError(t, checkDir.MkdirAll())

		// Make a tar archive with regular symlink maze
		outputTar := bytes.NewBuffer(nil)
		tw := tar.NewWriter(outputTar)
		addTarSymlink(t, tw, "aaa", "../secret")
		addTarSymlink(t, tw, "aaa/sym", "something")
		require.NoError(t, tw.Close())

		extractor := extract.Extractor{FS: &LoggingFS{}}
		require.Error(t, extractor.Tar(context.Background(), outputTar, targetDir.String(), nil))
		require.NoFileExists(t, checkDir.Join("sym").String())
	})

	t.Run("TarWithSymlinkToInternalPath", func(t *testing.T) {
		// Create target dir
		tmp := mkTempDir(t)
		targetDir := tmp.Join("test")
		require.NoError(t, targetDir.Mkdir())

		// Make a tar archive with regular symlink maze
		outputTar := bytes.NewBuffer(nil)
		tw := tar.NewWriter(outputTar)
		require.NoError(t, tw.WriteHeader(&tar.Header{Mode: 0o0777, Typeflag: tar.TypeDir, Name: "tmp"}))
		addTarSymlink(t, tw, "aaa", "tmp")
		addTarSymlink(t, tw, "aaa/sym", "something")
		require.NoError(t, tw.Close())

		extractor := extract.Extractor{FS: &LoggingFS{}}
		require.Error(t, extractor.Tar(context.Background(), outputTar, targetDir.String(), nil))
		require.NoFileExists(t, targetDir.Join("tmp", "sym").String())
	})

	t.Run("TarWithDoubleSymlinkToExternalPath", func(t *testing.T) {
		// Create target dir
		tmp := mkTempDir(t)
		targetDir := tmp.Join("test")
		require.NoError(t, targetDir.Mkdir())
		fmt.Println("TMP:", tmp)
		fmt.Println("TARGET DIR:", targetDir)

		// Make a tar archive with regular symlink maze
		outputTar := bytes.NewBuffer(nil)
		tw := tar.NewWriter(outputTar)
		tw.WriteHeader(&tar.Header{Name: "fake", Mode: 0777, Typeflag: tar.TypeDir})
		addTarSymlink(t, tw, "sym-maze", tmp.String())
		addTarSymlink(t, tw, "sym-maze", "fake")
		addTarSymlink(t, tw, "sym-maze/oops", "/tmp/something")
		require.NoError(t, tw.Close())

		extractor := extract.Extractor{FS: &LoggingFS{}}
		require.Error(t, extractor.Tar(context.Background(), outputTar, targetDir.String(), nil))
		require.NoFileExists(t, tmp.Join("oops").String())
	})

	t.Run("TarWithSymlinkToExternalPathWithoutMazing", func(t *testing.T) {
		// Create target dir
		tmp := mkTempDir(t)
		targetDir := tmp.Join("test")
		require.NoError(t, targetDir.Mkdir())

		// Make a tar archive with valid symlink maze
		outputTar := bytes.NewBuffer(nil)
		tw := tar.NewWriter(outputTar)
		require.NoError(t, tw.WriteHeader(&tar.Header{Mode: 0o0777, Typeflag: tar.TypeDir, Name: "tmp"}))
		addTarSymlink(t, tw, "aaa", "../tmp")
		require.NoError(t, tw.Close())

		extractor := extract.Extractor{FS: &LoggingFS{}}
		require.NoError(t, extractor.Tar(context.Background(), outputTar, targetDir.String(), nil))
		st, err := targetDir.Join("aaa").Lstat()
		require.NoError(t, err)
		require.Equal(t, "aaa", st.Name())
	})
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
				desiredPermString, _ := strings.CutPrefix(filename, "dir")
				desiredPerms, _ := strconv.ParseUint(desiredPermString, 8, 32)
				require.Equal(t, os.ModeDir|os.FileMode(OsDirPerms(desiredPerms)), info.Mode())
			} else if strings.HasPrefix(filename, "file") {
				desiredPermString, _ := strings.CutPrefix(filename, "file")
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
	download(t, "https://downloads.arduino.cc/libraries/github.com/arduino-libraries/LiquidCrystal-1.0.7.zip", archive)

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

func (m MockDisk) Remove(path string) error {
	return os.Remove(filepath.Join(m.Base, path))
}

func (m MockDisk) Stat(name string) (os.FileInfo, error) {
	name = filepath.Join(m.Base, name)
	return os.Stat(name)
}

func (m MockDisk) Chmod(name string, mode os.FileMode) error {
	name = filepath.Join(m.Base, name)
	return os.Chmod(name, mode)
}
