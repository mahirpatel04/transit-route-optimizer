package geocode

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/location"
)

// nycFilterBBox is "minLon,minLat,maxLon,maxLat", covering all five NYC
// boroughs, per the SearchPlaceIndexForText FilterBBox parameter format.
var nycFilterBBox = []float64{-74.26, 40.49, -73.68, 40.92}

// LocationServiceClient is the subset of the AWS Location Service client
// this package calls, so tests can substitute a fake.
type LocationServiceClient interface {
	SearchPlaceIndexForText(ctx context.Context, params *location.SearchPlaceIndexForTextInput, optFns ...func(*location.Options)) (*location.SearchPlaceIndexForTextOutput, error)
}

// LocationServiceGeocoder geocodes addresses via an AWS Location Service
// place index. Unlike the public Nominatim instance, it has no 1 req/sec
// cap, so no client-side throttling is needed here.
type LocationServiceGeocoder struct {
	client         LocationServiceClient
	placeIndexName string
}

// NewLocationServiceGeocoder loads AWS config from the environment (IAM role
// credentials in Lambda) and returns a geocoder backed by the given place
// index.
func NewLocationServiceGeocoder(ctx context.Context, placeIndexName string) (*LocationServiceGeocoder, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}
	return &LocationServiceGeocoder{
		client:         location.NewFromConfig(cfg),
		placeIndexName: placeIndexName,
	}, nil
}

func (g *LocationServiceGeocoder) Geocode(ctx context.Context, address string) (float64, float64, error) {
	out, err := g.client.SearchPlaceIndexForText(ctx, &location.SearchPlaceIndexForTextInput{
		IndexName:  aws.String(g.placeIndexName),
		Text:       aws.String(address),
		FilterBBox: nycFilterBBox,
		MaxResults: aws.Int32(1),
	})
	if err != nil {
		return 0, 0, fmt.Errorf("location service request failed: %w", err)
	}
	if len(out.Results) == 0 {
		return 0, 0, fmt.Errorf("%w for address %q", errNoResults, address)
	}

	point := out.Results[0].Place.Geometry.Point
	if len(point) != 2 {
		return 0, 0, fmt.Errorf("location service returned an unexpected geometry for address %q", address)
	}
	lon, lat := point[0], point[1]

	return lat, lon, nil
}
