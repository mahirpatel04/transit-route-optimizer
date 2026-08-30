package gtfs

import (
	"io"
	"log"
	"net/http"
)

func FetchData(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Data download failed: %v", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Data reading failed: %v", err)
	}

	return body, nil

}
