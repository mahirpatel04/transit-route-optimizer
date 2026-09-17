package geocode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type Geocoder interface {
	Geocode(ctx context.Context, address string) (lat, lon float64, err error)
}

// nycViewbox is "left,top,right,bottom" (min_lon,max_lat,max_lon,min_lat)
// covering all five NYC boroughs, per Nominatim's viewbox parameter format.
const nycViewbox = "-74.26,40.92,-73.68,40.49"

type NominatimGeocoder struct {
	httpClient *http.Client
	baseURL    string // overridable in tests; defaults to the real Nominatim host
}

func NewNominatimGeocoder(httpClient *http.Client) *NominatimGeocoder {
	return &NominatimGeocoder{
		httpClient: httpClient,
		baseURL:    "https://nominatim.openstreetmap.org",
	}
}

type nominatimResult struct {
	Lat string `json:"lat"`
	Lon string `json:"lon"`
}

func (g *NominatimGeocoder) Geocode(ctx context.Context, address string) (float64, float64, error) {
	params := url.Values{}
	params.Set("q", address)
	params.Set("format", "json")
	params.Set("limit", "1")
	params.Set("viewbox", nycViewbox)
	params.Set("bounded", "1")

	reqURL := g.baseURL + "/search?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to build geocode request: %w", err)
	}
	// Required by Nominatim's usage policy — requests without a descriptive
	// User-Agent may be blocked.
	req.Header.Set("User-Agent", "transit-route-optimizer/1.0 (github.com/mahirpatel04/transit-route-optimizer)")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("geocode request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("geocode request returned status %d", resp.StatusCode)
	}

	var results []nominatimResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return 0, 0, fmt.Errorf("failed to decode geocode response: %w", err)
	}
	if len(results) == 0 {
		return 0, 0, fmt.Errorf("no results for address %q", address)
	}

	lat, err := strconv.ParseFloat(results[0].Lat, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse latitude from geocode response: %w", err)
	}
	lon, err := strconv.ParseFloat(results[0].Lon, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse longitude from geocode response: %w", err)
	}

	return lat, lon, nil
}
