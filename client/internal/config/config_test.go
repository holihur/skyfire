package config

import (
	"reflect"
	"testing"
)

func TestNormalizeWhitelist(t *testing.T) {
	got := NormalizeWhitelist([]string{
		"  Api.Example.com ",
		"*.Example.COM.",
		"10.0.0.0/8",
		"10.0.0.0/8",
		"",
	})
	want := []string{"api.example.com", "*.example.com", "10.0.0.0/8"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeWhitelist = %v, want %v", got, want)
	}
}

func TestParseWhitelist(t *testing.T) {
	got := ParseWhitelist("api.example.com, *.corp.example\n10.0.0.0/8;1.1.1.1")
	want := []string{"api.example.com", "*.corp.example", "10.0.0.0/8", "1.1.1.1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseWhitelist = %v, want %v", got, want)
	}
}
