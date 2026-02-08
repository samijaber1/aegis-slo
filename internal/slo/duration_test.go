package slo

import (
	"testing"
	"time"
)

func TestParseDuration_Valid(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"1s", 1 * time.Second},
		{"30s", 30 * time.Second},
		{"1m", 1 * time.Minute},
		{"5m", 5 * time.Minute},
		{"1h", 1 * time.Hour},
		{"24h", 24 * time.Hour},
		{"1d", 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"30d", 30 * 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseDuration(tt.input)
			if err != nil {
				t.Fatalf("ParseDuration(%q) returned error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseDuration_Invalid(t *testing.T) {
	tests := []string{
		"",
		"invalid",
		"30",
		"30x",
		"30 s",
		"s30",
		"-5m",
		"1.5h",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := ParseDuration(input)
			if err == nil {
				t.Errorf("ParseDuration(%q) expected error, got nil", input)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{1 * time.Hour, "1h"},
		{24 * time.Hour, "1d"},
		{7 * 24 * time.Hour, "7d"},
		{90 * time.Second, "90s"},
		{90 * time.Minute, "90m"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatDuration(tt.input)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
