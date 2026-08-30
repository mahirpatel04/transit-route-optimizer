package main

import (
	"fmt"
	"log"

	"github.com/mahirpatel04/transit-route-optimizer/internal/gtfs"
)

func main() {
	body, err := gtfs.FetchData("https://rrgtfsfeeds.s3.amazonaws.com/gtfs_subway.zip")
	if err != nil {
		log.Fatalf("fetch failed: %v", err)
	}

	zr, err := gtfs.OpenZip(body)
	if err != nil {
		log.Fatalf("zip open failed: %v", err)
	}

	stopsData, err := gtfs.ReadFileFromZip(zr, "stops.txt")
	if err != nil {
		log.Fatalf("read stops.txt failed: %v", err)
	}

	fmt.Printf("stops.txt is %d bytes\n", len(stopsData))
}
