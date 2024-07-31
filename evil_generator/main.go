// This utility is used to generate the archives used as testdata for zipslip vulnerability
package main

//go:generate go run main.go ../testdata/zipslip

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"log"
	"os"

	"github.com/arduino/go-paths-helper"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Missing output directory")
	}
	outputDir := paths.New(os.Args[1])
	if outputDir.IsNotDir() {
		log.Fatalf("Output path %s is not a directory", outputDir)
	}

	generateEvilZipSlip(outputDir)
	generateEvilSymLinkPathTraversalTar(outputDir)
}

func generateEvilZipSlip(outputDir *paths.Path) {
	evilPathTraversalFiles := []string{
		"..",
		"../../../../../../../../../../../../../../../../../../../../tmp/evil.txt",
		"some/path/../../../../../../../../../../../../../../../../../../../../tmp/evil.txt",
		"/../../../../../../../../../../../../../../../../../../../../tmp/evil.txt",
		"/some/path/../../../../../../../../../../../../../../../../../../../../tmp/evil.txt",
	}
	winSpecificPathTraversalFiles := []string{
		"..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\tmp\\evil.txt",
		"some\\path\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\tmp\\evil.txt",
		"\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\tmp\\evil.txt",
		"\\some\\path\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\..\\tmp\\evil.txt",
	}
	winSpecificPathTraversalFiles = append(winSpecificPathTraversalFiles, evilPathTraversalFiles...)

	// Generate evil zip
	{
		buf := new(bytes.Buffer)
		w := zip.NewWriter(buf)
		for _, file := range winSpecificPathTraversalFiles {
			if f, err := w.Create(file); err != nil {
				log.Fatal(err)
			} else if _, err = f.Write([]byte("TEST")); err != nil {
				log.Fatal(err)
			}
		}
		if err := w.Close(); err != nil {
			log.Fatal(err)
		}
		if err := outputDir.Join("evil.zip").WriteFile(buf.Bytes()); err != nil {
			log.Fatal(err)
		}
	}

	// Generate evil tar
	{
		buf := new(bytes.Buffer)
		w := tar.NewWriter(buf)
		for _, file := range evilPathTraversalFiles {
			if err := w.WriteHeader(&tar.Header{
				Name: file,
				Size: 4,
				Mode: 0666,
			}); err != nil {
				log.Fatal(err)
			}
			if _, err := w.Write([]byte("TEST")); err != nil {
				log.Fatal(err)
			}
		}
		if err := w.Close(); err != nil {
			log.Fatal(err)
		}
		if err := outputDir.Join("evil.tar").WriteFile(buf.Bytes()); err != nil {
			log.Fatal(err)
		}
	}

	// Generate evil tar for windows
	{
		buf := new(bytes.Buffer)
		w := tar.NewWriter(buf)
		for _, file := range winSpecificPathTraversalFiles {
			if err := w.WriteHeader(&tar.Header{
				Name: file,
				Size: 4,
				Mode: 0666,
			}); err != nil {
				log.Fatal(err)
			}
			if _, err := w.Write([]byte("TEST")); err != nil {
				log.Fatal(err)
			}
		}
		if err := w.Close(); err != nil {
			log.Fatal(err)
		}
		if err := outputDir.Join("evil-win.tar").WriteFile(buf.Bytes()); err != nil {
			log.Fatal(err)
		}
	}
}

func generateEvilSymLinkPathTraversalTar(outputDir *paths.Path) {
	outputTarFile, err := outputDir.Join("evil-link-traversal.tar").Create()
	if err != nil {
		log.Fatal(err)
	}
	defer outputTarFile.Close()

	tw := tar.NewWriter(outputTarFile)
	defer tw.Close()

	if err := tw.WriteHeader(&tar.Header{
		Name: "leak", Linkname: "../../../../../../../../../../../../../../../tmp/something-important",
		Mode: 0o0777, Size: 0, Typeflag: tar.TypeLink,
	}); err != nil {
		log.Fatal(err)
	}
}
