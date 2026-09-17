// cmd/ingest/main_test.go
package main

import "testing"

func TestIsCronEvent(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{
			name: "eventbridge scheduled event",
			raw:  `{"source":"aws.events","detail-type":"Scheduled Event","detail":{}}`,
			want: true,
		},
		{
			name: "function url http request",
			raw:  `{"version":"2.0","routeKey":"$default","rawPath":"/routes","requestContext":{"http":{"method":"GET"}}}`,
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isCronEvent([]byte(tc.raw))
			if got != tc.want {
				t.Errorf("isCronEvent(%s) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}
