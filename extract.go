// Package extract allows to extract archives in zip, tar.gz or tar.bz2 formats
// easily.
//
// Most of the time you'll just need to call the proper function with a buffer and
// a destination:
//
//     data, _ := ioutil.ReadFile("path/to/file.tar.bz2")
//     buffer := bytes.NewBuffer(data)
//     extract.TarBz2(data, "/path/where/to/extract", nil)
//
// Sometimes you'll want a bit more control over the files, such as extracting
// a subfolder of the archive. In this cases you can specify a renamer	func
// that will change the path for every file:
//
//      var shift = func(path string) string {
//          parts := strings.Split(path, string(filepath.Separator))
//          parts = parts[1:]
//          return strings.Join(parts, string(filepath.Separator))
//      }
//      extract.TarBz2(data, "/path/where/to/extract", shift)
package extract

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/juju/errors"
)

// Renamer is a function that can be used to rename the files when you're extracting
// them. For example you may want to only extract files with a certain pattern.
// If you return an empty string they won't be extracted.
type Renamer func(string) string

// TarBz2 extracts a .tar.bz2 archived stream of data in the specified location.
// It accepts a rename function to handle the names of the files (see the example)
func TarBz2(body io.Reader, location string, rename Renamer) error {
	reader := bzip2.NewReader(body)
	return Tar(reader, location, rename)
}

// TarGz extracts a .tar.gz archived stream of data in the specified location.
// It accepts a rename function to handle the names of the files (see the example)
func TarGz(body io.Reader, location string, rename Renamer) error {
	reader, err := gzip.NewReader(body)
	if err != nil {
		return errors.Annotatef(err, "Gunzip")
	}
	return Tar(reader, location, rename)
}

type file struct {
	Path string
	Mode os.FileMode
	Data bytes.Buffer
}
type link struct {
	Name string
	Path string
}

// Tar extracts a .tar archived stream of data in the specified location.
// It accepts a rename function to handle the names of the files (see the example)
func Tar(body io.Reader, location string, rename Renamer) error {
	files := []file{}
	links := []link{}
	symlinks := []link{}

	// We make the first pass creating the directory structure, or we could end up
	// attempting to create a file where there's no folder
	tr := tar.NewReader(body)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return errors.Annotatef(err, "Read tar stream")
		}

		path := header.Name
		if rename != nil {
			path = rename(path)
		}

		if path == "" {
			continue
		}

		path = filepath.Join(location, path)
		info := header.FileInfo()

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, info.Mode()); err != nil {
				return errors.Annotatef(err, "Create directory %s", path)
			}
		case tar.TypeReg, tar.TypeRegA:
			var data bytes.Buffer
			if _, err := io.Copy(&data, tr); err != nil {
				return errors.Annotatef(err, "Read contents of file %s", path)
			}
			files = append(files, file{Path: path, Mode: info.Mode(), Data: data})
		case tar.TypeLink:
			name := header.Linkname
			if rename != nil {
				name = rename(name)
			}

			name = filepath.Join(location, name)
			links = append(links, link{Path: path, Name: name})
		case tar.TypeSymlink:
			symlinks = append(symlinks, link{Path: path, Name: header.Linkname})
		}
	}

	// Now we make another pass creating the files and links
	for i := range files {
		if err := copy(files[i].Path, files[i].Mode, &files[i].Data); err != nil {
			return errors.Annotatef(err, "Create file %s", files[i].Path)
		}
	}

	for i := range links {
		if err := os.Link(links[i].Name, links[i].Path); err != nil {
			return errors.Annotatef(err, "Create link %s", links[i].Path)
		}
	}

	for i := range symlinks {
		if err := os.Symlink(symlinks[i].Name, symlinks[i].Path); err != nil {
			return errors.Annotatef(err, "Create link %s", symlinks[i].Path)
		}
	}
	return nil
}

// Zip extracts a .zip archived stream of data in the specified location.
// It accepts a rename function to handle the names of the files (see the example).
func Zip(body io.Reader, location string, rename Renamer) error {
	// read the whole body into a buffer. Not sure this is the best way to do it
	buffer := bytes.NewBuffer([]byte{})
	io.Copy(buffer, body)

	archive, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		return errors.Annotatef(err, "Read the zip file")
	}

	files := []file{}
	links := []link{}

	// We make the first pass creating the directory structure, or we could end up
	// attempting to create a file where there's no folder
	for _, header := range archive.File {
		path := header.Name
		if rename != nil {
			path = rename(path)
		}

		if path == "" {
			continue
		}

		path = filepath.Join(location, path)
		info := header.FileInfo()

		switch {
		case info.IsDir():
			if err := os.MkdirAll(path, info.Mode()); err != nil {
				return errors.Annotatef(err, "Create directory %s", path)
			}
		// We only check for symlinks because hard links aren't possible
		case info.Mode()&os.ModeSymlink != 0:
			f, err := header.Open()
			if err != nil {
				return errors.Annotatef(err, "Open link %s", path)
			}
			name, err := ioutil.ReadAll(f)
			if err != nil {
				return errors.Annotatef(err, "Read address of link %s", path)
			}
			links = append(links, link{Path: path, Name: string(name)})
		default:
			f, err := header.Open()
			if err != nil {
				return errors.Annotatef(err, "Open file %s", path)
			}
			var data bytes.Buffer
			if _, err := io.Copy(&data, f); err != nil {
				return errors.Annotatef(err, "Read contents of file %s", path)
			}
			files = append(files, file{Path: path, Mode: info.Mode(), Data: data})
		}
	}

	// Now we make another pass creating the files and links
	for i := range files {
		if err := copy(files[i].Path, files[i].Mode, &files[i].Data); err != nil {
			return errors.Annotatef(err, "Create file %s", files[i].Path)
		}
	}

	for i := range links {
		if err := os.Symlink(links[i].Name, links[i].Path); err != nil {
			return errors.Annotatef(err, "Create link %s", links[i].Path)
		}
	}

	return nil
}

func copy(path string, mode os.FileMode, src io.Reader) error {
	err := os.MkdirAll(filepath.Dir(path), mode|os.ModeDir)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, src)
	return err
}
