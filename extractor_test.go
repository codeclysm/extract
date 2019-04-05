package extract_test

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/codeclysm/extract"
)

func TestExtractor_Tar(t *testing.T) {
	tmp, _ := ioutil.TempDir("", "")

	extractor := extract.Extractor{
		FS: MockDisk{
			Base: tmp,
		},
	}

	data, err := ioutil.ReadFile("testdata/archive.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	buffer := bytes.NewBuffer(data)

	err = extractor.Gz(context.Background(), buffer, "/", nil)
	if err != nil {
		t.Error(err.Error())
	}

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
	testWalk(t, tmp, files)
}

func TestExtractor_Zip(t *testing.T) {
	tmp, _ := ioutil.TempDir("", "")

	extractor := extract.Extractor{
		FS: MockDisk{
			Base: tmp,
		},
	}

	data, err := ioutil.ReadFile("testdata/archive.zip")
	if err != nil {
		t.Fatal(err)
	}
	buffer := bytes.NewBuffer(data)

	err = extractor.Zip(context.Background(), buffer, "/", nil)
	if err != nil {
		t.Error(err.Error())
	}

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
	testWalk(t, tmp, files)
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
