package service

import (
	"archive/zip"
	"io"
	"log"
	"os"
)

func CreateArchive(filePaths []string, fileNames []string, dest string) error {
	archive, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func(archive *os.File) {
		err = archive.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(archive)

	zipWriter := zip.NewWriter(archive)
	defer func(zipWriter *zip.Writer) {
		err = zipWriter.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(zipWriter)

	for i, path := range filePaths {
		if path == "" {
			continue
		}

		file, err := os.Open(path)
		if err != nil {
			continue
		}
		defer func(file *os.File) {
			err = file.Close()
			if err != nil {
				log.Fatal(err)
			}
		}(file)

		header := &zip.FileHeader{
			Name:   fileNames[i],
			Method: zip.Deflate,
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			continue
		}

		if _, err := io.Copy(writer, file); err != nil {
			continue
		}
	}

	return nil
}
