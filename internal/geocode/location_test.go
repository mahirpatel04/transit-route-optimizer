package geocode

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/location"
	"github.com/aws/aws-sdk-go-v2/service/location/types"
)

type fakeLocationClient struct {
	output   *location.SearchPlaceIndexForTextOutput
	err      error
	gotInput *location.SearchPlaceIndexForTextInput
}

func (f *fakeLocationClient) SearchPlaceIndexForText(ctx context.Context, params *location.SearchPlaceIndexForTextInput, optFns ...func(*location.Options)) (*location.SearchPlaceIndexForTextOutput, error) {
	f.gotInput = params
	return f.output, f.err
}

func TestLocationServiceGeocoder_ParsesSuccessfulResponse(t *testing.T) {
	fake := &fakeLocationClient{
		output: &location.SearchPlaceIndexForTextOutput{
			Results: []types.SearchForTextResult{
				{Place: &types.Place{Geometry: &types.PlaceGeometry{Point: []float64{-73.9855, 40.7580}}}},
			},
		},
	}
	g := &LocationServiceGeocoder{client: fake, placeIndexName: "test-index"}

	lat, lon, err := g.Geocode(context.Background(), "Times Square, NYC")
	if err != nil {
		t.Fatalf("Geocode: %v", err)
	}
	if lat != 40.7580 || lon != -73.9855 {
		t.Errorf("got (%v, %v), want (40.7580, -73.9855)", lat, lon)
	}
	if aws.ToString(fake.gotInput.IndexName) != "test-index" {
		t.Errorf("expected the configured index name to be used, got %q", aws.ToString(fake.gotInput.IndexName))
	}
}

func TestLocationServiceGeocoder_NoResultsReturnsError(t *testing.T) {
	fake := &fakeLocationClient{output: &location.SearchPlaceIndexForTextOutput{}}
	g := &LocationServiceGeocoder{client: fake, placeIndexName: "test-index"}

	_, _, err := g.Geocode(context.Background(), "somewhere that doesn't exist")
	if err == nil {
		t.Fatal("expected an error for zero results")
	}
	if !strings.Contains(err.Error(), "no results") {
		t.Errorf("expected error to mention 'no results', got: %v", err)
	}
}
