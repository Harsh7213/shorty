package controllers

import (
	"net/http/httptest"
	"testing"
)

func TestRequestCountryCode(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   any
	}{
		{name: "country code", header: "us", want: "US"},
		{name: "unknown country", header: "XX"},
		{name: "tor exit", header: "T1"},
		{name: "invalid country code", header: "USA"},
		{name: "invalid characters", header: "U1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "/", nil)
			if test.header != "" {
				request.Header.Set("CF-IPCountry", test.header)
			}
			if got := requestCountryCode(request); got != test.want {
				t.Errorf("requestCountryCode() = %v, want %v", got, test.want)
			}
		})
	}
}
