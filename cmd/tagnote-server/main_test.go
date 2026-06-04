package main

import "testing"

func TestWebGoogleClientID(t *testing.T) {
	const web = "561303077556-web.apps.googleusercontent.com"
	const ios = "561303077556-ios.apps.googleusercontent.com"

	cases := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty", raw: "", want: ""},
		{name: "single web id", raw: web, want: web},
		// Regression: a comma-separated audience list must inject ONLY the web
		// client ID in the browser, never the whole list (which Google rejects
		// with "invalid_client").
		{name: "web and ios", raw: web + "," + ios, want: web},
		{name: "spaces around entries", raw: " " + web + " , " + ios + " ", want: web},
		{name: "trailing comma", raw: web + ",", want: web},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := webGoogleClientID(tc.raw); got != tc.want {
				t.Fatalf("webGoogleClientID(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}
