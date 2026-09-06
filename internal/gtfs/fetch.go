package gtfs

import (
	"fmt"
	"io"
	"net/http"
)

func FetchData(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("data download failed: %w", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("data reading failed: %w", err)
	}

	return body, nil

}
