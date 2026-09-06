package gtfs

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
)

func OpenZip(data []byte) (*zip.Reader, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}

	return reader, nil
}

func ReadFileFromZip(zr *zip.Reader, filename string) ([]byte, error) {
	for _, f := range zr.File {
		if f.Name == filename {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open %s: %w", filename, err)
			}

			defer rc.Close()

			data, err := io.ReadAll(rc)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", filename, err)
			}

			return data, nil
		}
	}
	return nil, fmt.Errorf("file %s not found in zip", filename)
}
