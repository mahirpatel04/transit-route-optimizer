package geocode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNominatimGeocoder_ParsesSuccessfulResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); !strings.Contains(got, "transit-route-optimizer") {
			t.Errorf("expected a descriptive User-Agent, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"lat":"40.7580","lon":"-73.9855"}]`))
	}))
	defer server.Close()

	g := NewNominatimGeocoder(server.Client())
	g.baseURL = server.URL

	lat, lon, err := g.Geocode(context.Background(), "Times Square, NYC")
	if err != nil {
		t.Fatalf("Geocode: %v", err)
	}
	if lat != 40.7580 || lon != -73.9855 {
		t.Errorf("got (%v, %v), want (40.7580, -73.9855)", lat, lon)
	}
}

func TestNominatimGeocoder_NoResultsReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	g := NewNominatimGeocoder(server.Client())
	g.baseURL = server.URL

	_, _, err := g.Geocode(context.Background(), "somewhere that doesn't exist")
	if err == nil {
		t.Fatal("expected an error for zero results")
	}
	if !strings.Contains(err.Error(), "no results") {
		t.Errorf("expected error to mention 'no results', got: %v", err)
	}
}

func TestNominatimGeocoder_BoundsToNYC(t *testing.T) {
	var capturedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"lat":"40.7580","lon":"-73.9855"}]`))
	}))
	defer server.Close()

	g := NewNominatimGeocoder(server.Client())
	g.baseURL = server.URL

	_, _, err := g.Geocode(context.Background(), "Main St")
	if err != nil {
		t.Fatalf("Geocode: %v", err)
	}
	if !strings.Contains(capturedQuery, "viewbox=") || !strings.Contains(capturedQuery, "bounded=1") {
		t.Errorf("expected the request to include a viewbox and bounded=1, got query: %s", capturedQuery)
	}
}
