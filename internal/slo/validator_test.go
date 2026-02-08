package slo

import (
	"path/filepath"
	"testing"
)

func TestValidator_ValidateDirectory_ValidFiles(t *testing.T) {
	validator := mustNewValidator(t)

	errors := validator.ValidateDirectory("../../fixtures/slo/valid")

	if len(errors) != 0 {
		t.Errorf("expected no errors, got %d:", len(errors))
		for _, err := range errors {
			t.Logf("  %v", err)
		}
	}
}

func TestValidator_ValidateDirectory_InvalidFiles(t *testing.T) {
	validator := mustNewValidator(t)

	errors := validator.ValidateDirectory("../../fixtures/slo/invalid")

	if len(errors) == 0 {
		t.Fatal("expected validation errors, got none")
	}

	t.Logf("Got %d total errors", len(errors))
	for _, err := range errors {
		t.Logf("Error: %s: %s: %s", filepath.Base(err.File), err.Path, err.Message)
	}

	// Group errors by file
	errorsByFile := make(map[string][]ValidationError)
	for _, err := range errors {
		base := filepath.Base(err.File)
		errorsByFile[base] = append(errorsByFile[base], err)
	}

	// Test missing-fields.yaml
	if errs, ok := errorsByFile["missing-fields.yaml"]; ok {
		if len(errs) == 0 {
			t.Error("expected errors for missing-fields.yaml")
		}
		// Should have error about missing objective field
		hasObjectiveError := false
		for _, err := range errs {
			if contains(err.Message, "objective") || contains(err.Path, "objective") {
				hasObjectiveError = true
				break
			}
		}
		if !hasObjectiveError {
			t.Error("expected error about missing objective field")
		}
	} else {
		t.Error("expected errors for missing-fields.yaml")
	}

	// Test compliance-too-small.yaml
	if errs, ok := errorsByFile["compliance-too-small.yaml"]; ok {
		if len(errs) == 0 {
			t.Error("expected errors for compliance-too-small.yaml")
		}
		// Should have error about compliance window
		hasComplianceError := false
		for _, err := range errs {
			if contains(err.Message, "complianceWindow") && contains(err.Message, "burn policy") {
				hasComplianceError = true
				break
			}
		}
		if !hasComplianceError {
			t.Errorf("expected error about compliance window being too small, got: %v", errs)
		}
	} else {
		t.Error("expected errors for compliance-too-small.yaml")
	}

	// Test duplicate IDs
	hasDuplicateError := false
	for _, errs := range errorsByFile {
		for _, err := range errs {
			if contains(err.Message, "duplicate") || contains(err.Message, "dup-slo") {
				hasDuplicateError = true
				break
			}
		}
		if hasDuplicateError {
			break
		}
	}
	if !hasDuplicateError {
		t.Error("expected error about duplicate IDs")
	}
}

func TestValidator_ValidateDirectory_MixedFiles(t *testing.T) {
	validator := mustNewValidator(t)

	errors := validator.ValidateDirectory("../../fixtures/slo")

	// Should have errors from invalid directory, but valid directory should pass
	if len(errors) == 0 {
		t.Fatal("expected validation errors from invalid files, got none")
	}

	// Check that we only have errors from invalid files
	for _, err := range errors {
		if contains(err.File, "valid") && !contains(err.File, "invalid") {
			t.Errorf("unexpected error from valid file: %v", err)
		}
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		wantSecs int64
		wantErr  bool
	}{
		{"30s", 30, false},
		{"5m", 300, false},
		{"1h", 3600, false},
		{"30d", 30 * 24 * 3600, false},
		{"invalid", 0, true},
		{"", 0, true},
		{"30", 0, true},
		{"30x", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseDuration(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseDuration(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseDuration(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got.Seconds() != float64(tt.wantSecs) {
				t.Errorf("ParseDuration(%q) = %v seconds, want %d seconds", tt.input, got.Seconds(), tt.wantSecs)
			}
		})
	}
}

func TestLoadFromDirectory(t *testing.T) {
	sloWithFiles, errors := LoadFromDirectory("../../fixtures/slo/valid")

	if len(errors) != 0 {
		t.Errorf("expected no load errors, got %d:", len(errors))
		for _, err := range errors {
			t.Logf("  %v", err)
		}
	}

	if len(sloWithFiles) == 0 {
		t.Fatal("expected to load SLOs, got none")
	}

	// Verify structure of loaded SLO
	slo := sloWithFiles[0].SLO
	if slo.APIVersion != "aegis.dev/v1" {
		t.Errorf("expected apiVersion = aegis.dev/v1, got %s", slo.APIVersion)
	}
	if slo.Kind != "SLO" {
		t.Errorf("expected kind = SLO, got %s", slo.Kind)
	}
	if slo.Metadata.ID == "" {
		t.Error("expected metadata.id to be set")
	}
	if slo.Spec.Objective <= 0 || slo.Spec.Objective >= 1 {
		t.Errorf("expected objective in (0,1), got %f", slo.Spec.Objective)
	}
	if sloWithFiles[0].File == "" {
		t.Error("expected file path to be set")
	}
}

func TestValidateComplianceWindow(t *testing.T) {
	tests := []struct {
		name        string
		compliance  string
		shortWindow string
		longWindow  string
		expectError bool
	}{
		{
			name:        "valid - compliance > long window",
			compliance:  "30d",
			shortWindow: "5m",
			longWindow:  "1h",
			expectError: false,
		},
		{
			name:        "invalid - compliance < long window",
			compliance:  "1h",
			shortWindow: "30m",
			longWindow:  "6h",
			expectError: true,
		},
		{
			name:        "valid - compliance = long window",
			compliance:  "6h",
			shortWindow: "30m",
			longWindow:  "6h",
			expectError: false,
		},
		{
			name:        "invalid - compliance < short window",
			compliance:  "1h",
			shortWindow: "6h",
			longWindow:  "12h",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slo := &SLO{
				Spec: Spec{
					ComplianceWindow: tt.compliance,
					BurnPolicy: BurnPolicy{
						Rules: []BurnRule{
							{
								ShortWindow: tt.shortWindow,
								LongWindow:  tt.longWindow,
							},
						},
					},
				},
			}

			errors := validateComplianceWindow("test.yaml", slo)

			hasError := len(errors) > 0
			if hasError != tt.expectError {
				t.Errorf("expected error=%v, got error=%v (errors: %v)", tt.expectError, hasError, errors)
			}
		})
	}
}

// Helper functions

func mustNewValidator(t *testing.T) *Validator {
	t.Helper()
	validator, err := NewValidator("../../schemas/slo_v1.json")
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}
	return validator
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
