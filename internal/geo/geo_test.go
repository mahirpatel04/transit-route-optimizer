package geo

import (
	"math"
	"testing"
)

func TestHaversine_SamePoint(t *testing.T) {
	d := Haversine(40.7580, -73.9855, 40.7580, -73.9855)
	if d != 0 {
		t.Errorf("expected 0 distance for identical points, got %f", d)
	}
}

func TestHaversine_OneDegreeAtEquator(t *testing.T) {
	d := Haversine(0, 0, 0, 1)
	want := 111195.0
	if math.Abs(d-want) > 1000 {
		t.Errorf("expected ~%.0fm, got %.0fm", want, d)
	}
}

func TestHaversine_KnownNYCDistance(t *testing.T) {
	// Times Square (40.7580, -73.9855) to Grand Central (40.7527, -73.9772),
	// roughly 900m apart.
	d := Haversine(40.7580, -73.9855, 40.7527, -73.9772)
	if d < 700 || d > 1200 {
		t.Errorf("expected roughly 700-1200m between Times Square and Grand Central, got %.0fm", d)
	}
}
