package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
)

func TestCountryEndpointReturnsRowsForCountryPicker(t *testing.T) {
	for _, test := range []struct {
		name, field string
		codes       []string
	}{
		{"all", "", []string{"86", "852", "853", "886", "65", "60", "1"}},
		{"dial code search", "852", []string{"852"}},
		{"name search", "MALAYSIA", []string{"60"}},
		{"no matches", "no-such-country", []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/appapi/?service=Login.getCountrys&field="+url.QueryEscape(test.field), nil)
			recorder := httptest.NewRecorder()
			(&Server{}).compat(recorder, request)
			var envelope struct {
				Data struct {
					Code int `json:"code"`
					Info []struct {
						Name string `json:"name"`
						Tel  string `json:"tel"`
					} `json:"info"`
				} `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("country picker requires rows directly in data.info: %v", err)
			}
			if recorder.Code != http.StatusOK || envelope.Data.Code != 0 || envelope.Data.Info == nil {
				t.Fatalf("invalid country list response: %s", recorder.Body.String())
			}
			codes := make([]string, 0, len(envelope.Data.Info))
			for _, row := range envelope.Data.Info {
				if row.Name == "" {
					t.Fatal("country label missing")
				}
				codes = append(codes, row.Tel)
			}
			if !reflect.DeepEqual(codes, test.codes) {
				t.Fatalf("country codes = %v, want %v", codes, test.codes)
			}
		})
	}
}
