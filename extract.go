// Package extract allows to extract archives in zip, tar.gz or tar.bz2 formats
// easily.
//
// Most of the time you'll just need to call the proper function with a buffer and
// a destination:
//
// 	data, _ := ioutil.ReadFile("path/to/file.tar.bz2")
// 	buffer := bytes.NewBuffer(data)
// 	extract.TarBz2(data, "/path/where/to/extract", nil)
// ```
//
// Sometimes you'll want a bit more control over the files, such as extracting a
// subfolder of the archive. In this cases you can specify a renamer func that will
// change the path for every file:
//
// 	var shift = func(path string) string {
//		parts := strings.Split(path, string(filepath.Separator))
//		parts = parts[1:]
//		return strings.Join(parts, string(filepath.Separator))
//	}
//	extract.TarBz2(data, "/path/where/to/extract", shift)
// ```
//
// If you don't know which archive you're dealing with (life really is always a surprise) you can use Archive, which will infer the type of archive from the first bytes
//
// 	extract.Archive(data, "/path/where/to/extract", nil)
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

	filetype "gopkg.in/h2non/filetype.v1"
	"gopkg.in/h2non/filetype.v1/types"

	"github.com/juju/errors"
)

// Renamer is a function that can be used to rename the files when you're extracting
// them. For example you may want to only extract files with a certain pattern.
// If you return an empty string they won't be extracted.
type Renamer func(string) string

// Archive extracts a generic archived stream of data in the specified location.
// It automatically detects the archive type and accepts a rename function to
// handle the names of the files.
// If the file is not an archive, an error is returned.
func Archive(body io.Reader, location string, rename Renamer) error {
	dummy := make(chan bool, 1)
	defer close(dummy)
	return ArchiveCancel(body, location, rename, dummy)
}

// ArchiveCancel is the same as Archive but with an extra channel to stop
// the extraction.
func ArchiveCancel(body io.Reader, location string, rename Renamer, cancel <-chan bool) error {
	body, kind, err := match(body)
	if err != nil {
		errors.Annotatef(err, "Detect archive type")
	}

	switch kind.Extension {
	case "zip":
		return ZipCancel(body, location, rename, cancel)
	case "gz":
		return GzCancel(body, location, rename, cancel)
	case "bz2":
		return Bz2Cancel(body, location, rename, cancel)
	case "tar":
		return TarCancel(body, location, rename, cancel)
	default:
		return errors.New("Not a supported archive")
	}
}

// Bz2 extracts a .bz2 or .tar.bz2 archived stream of data in the specified location.
// It accepts a rename function to handle the names of the files (see the example)
func Bz2(body io.Reader, location string, rename Renamer) error {
	dummy := make(chan bool, 1)
	defer close(dummy)
	return Bz2Cancel(body, location, rename, dummy)
}

// Bz2Cancel is the same as Bz2 but with an extra channel to stop the extraction.
func Bz2Cancel(body io.Reader, location string, rename Renamer, cancel <-chan bool) error {
	reader := bzip2.NewReader(body)

	body, kind, err := match(reader)
	if err != nil {
		return errors.Annotatef(err, "extract bz2: detect")
	}

	if kind.Extension == "tar" {
		return TarCancel(body, location, rename, cancel)
	}

	err = copy(location, 0666, body, cancel)
	if err != nil {
		return err
	}
	return nil
}

// Gz extracts a .gz or .tar.gz archived stream of data in the specified location.
// It accepts a rename function to handle the names of the files (see the example)
func Gz(body io.Reader, location string, rename Renamer) error {
	dummy := make(chan bool, 1)
	defer close(dummy)
	return GzCancel(body, location, rename, dummy)
}

// GzCancel is the same as Gz but with an extra channel to stop the extraction.
func GzCancel(body io.Reader, location string, rename Renamer, cancel <-chan bool) error {
	reader, err := gzip.NewReader(body)
	if err != nil {
		return errors.Annotatef(err, "Gunzip")
	}

	body, kind, err := match(reader)
	if err != nil {
		return err
	}

	if kind.Extension == "tar" {
		return TarCancel(body, location, rename, cancel)
	}
	err = copy(location, 0666, body, cancel)
	if err != nil {
		return err
	}
	return nil
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
	dummy := make(chan bool, 1)
	defer close(dummy)
	return TarCancel(body, location, rename, dummy)
}

// TarCancel is the same as Tar but with an extra channel to stop the extraction.
func TarCancel(body io.Reader, location string, rename Renamer, cancel <-chan bool) error {
	files := []file{}
	links := []link{}
	symlinks := []link{}

	// We make the first pass creating the directory structure, or we could end up
	// attempting to create a file where there's no folder
	tr := tar.NewReader(body)
	for {
		select {
		case <-cancel:
			return errors.New("interrupted")
		default:
		}

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
			if _, err := copyCancel(&data, tr, cancel); err != nil {
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
		if err := copy(files[i].Path, files[i].Mode, &files[i].Data, cancel); err != nil {
			return errors.Annotatef(err, "Create file %s", files[i].Path)
		}
	}

	for i := range links {
		select {
		case <-cancel:
			return errors.New("interrupted")
		default:
		}
		if err := os.Link(links[i].Name, links[i].Path); err != nil {
			return errors.Annotatef(err, "Create link %s", links[i].Path)
		}
	}

	for i := range symlinks {
		select {
		case <-cancel:
			return errors.New("interrupted")
		default:
		}
		if err := os.Symlink(symlinks[i].Name, symlinks[i].Path); err != nil {
			return errors.Annotatef(err, "Create link %s", symlinks[i].Path)
		}
	}
	return nil
}

// Zip extracts a .zip archived stream of data in the specified location.
// It accepts a rename function to handle the names of the files (see the example).
func Zip(body io.Reader, location string, rename Renamer) error {
	dummy := make(chan bool, 1)
	defer close(dummy)
	return ZipCancel(body, location, rename, dummy)
}

// ZipCancel is the same as Bz2 but with an extra channel to stop the extraction.
func ZipCancel(body io.Reader, location string, rename Renamer, cancel <-chan bool) error {
	// read the whole body into a buffer. Not sure this is the best way to do it
	buffer := bytes.NewBuffer([]byte{})
	copyCancel(buffer, body, cancel)

	archive, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		return errors.Annotatef(err, "Read the zip file")
	}

	files := []file{}
	links := []link{}

	// We make the first pass creating the directory structure, or we could end up
	// attempting to create a file where there's no folder
	for _, header := range archive.File {
		select {
		case <-cancel:
			return errors.New("interrupted")
		default:
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

		switch {
		case info.IsDir():
			if err := os.MkdirAll(path, info.Mode()|os.ModeDir|100); err != nil {
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
			if _, err := copyCancel(&data, f, cancel); err != nil {
				return errors.Annotatef(err, "Read contents of file %s", path)
			}
			files = append(files, file{Path: path, Mode: info.Mode(), Data: data})
		}
	}

	// Now we make another pass creating the files and links
	for i := range files {
		if err := copy(files[i].Path, files[i].Mode, &files[i].Data, cancel); err != nil {
			return errors.Annotatef(err, "Create file %s", files[i].Path)
		}
	}

	for i := range links {
		select {
		case <-cancel:
			return errors.New("interrupted")
		default:
		}
		if err := os.Symlink(links[i].Name, links[i].Path); err != nil {
			return errors.Annotatef(err, "Create link %s", links[i].Path)
		}
	}

	return nil
}

func copy(path string, mode os.FileMode, src io.Reader, cancel <-chan bool) error {
	// We add the execution permission to be able to create files inside it
	err := os.MkdirAll(filepath.Dir(path), mode|os.ModeDir|100)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = copyCancel(file, src, cancel)
	return err
}

// match reads the first 512 bytes, calls types.Match and returns a reader
// for the whole stream
func match(r io.Reader) (io.Reader, types.Type, error) {
	buffer := make([]byte, 512)

	n, err := r.Read(buffer)
	if err != nil && err != io.EOF {
		return nil, types.Unknown, err
	}

	r = io.MultiReader(bytes.NewBuffer(buffer[:n]), r)

	typ, err := filetype.Match(buffer)

	return r, typ, err
}
