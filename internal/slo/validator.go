package slo

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

// Validator handles SLO validation
type Validator struct {
	schema *jsonschema.Schema
}

// NewValidator creates a new validator with the given schema file
func NewValidator(schemaPath string) (*Validator, error) {
	compiler := jsonschema.NewCompiler()

	// Load schema from file path
	// The schema will be auto-detected based on $schema field
	schema, err := compiler.Compile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}

	return &Validator{schema: schema}, nil
}

// ValidateDirectory loads and validates all SLO files in a directory
func (v *Validator) ValidateDirectory(dirPath string) []ValidationError {
	sloWithFiles, loadErrors := LoadFromDirectory(dirPath)

	var allErrors []ValidationError
	allErrors = append(allErrors, loadErrors...)

	if len(sloWithFiles) == 0 {
		return allErrors
	}

	// Validate each SLO against JSON schema
	for _, sloWithFile := range sloWithFiles {
		schemaErrors := v.validateSchema(sloWithFile.File, sloWithFile.SLO)
		allErrors = append(allErrors, schemaErrors...)
	}

	// Apply extra validation rules
	extraErrors := v.validateExtraRules(sloWithFiles)
	allErrors = append(allErrors, extraErrors...)

	return allErrors
}

// validateSchema validates a single SLO against the JSON schema
func (v *Validator) validateSchema(file string, slo *SLO) []ValidationError {
	var errors []ValidationError

	// Convert SLO to JSON for schema validation
	yamlBytes, err := yaml.Marshal(slo)
	if err != nil {
		errors = append(errors, ValidationError{
			File:    file,
			Message: fmt.Sprintf("failed to marshal SLO: %v", err),
		})
		return errors
	}

	var jsonData interface{}
	if err := yaml.Unmarshal(yamlBytes, &jsonData); err != nil {
		errors = append(errors, ValidationError{
			File:    file,
			Message: fmt.Sprintf("failed to convert to JSON: %v", err),
		})
		return errors
	}

	// Validate against schema
	if err := v.schema.Validate(jsonData); err != nil {
		if validationErr, ok := err.(*jsonschema.ValidationError); ok {
			errors = append(errors, extractSchemaErrors(file, validationErr)...)
		} else {
			errors = append(errors, ValidationError{
				File:    file,
				Message: err.Error(),
			})
		}
	}

	return errors
}

// extractSchemaErrors converts JSON schema validation errors to ValidationErrors
func extractSchemaErrors(file string, err *jsonschema.ValidationError) []ValidationError {
	var errors []ValidationError

	// Add the main error
	path := strings.Join(err.InstanceLocation, ".")
	if path == "" {
		path = "(root)"
	}

	errors = append(errors, ValidationError{
		File:    file,
		Path:    path,
		Message: err.Error(),
	})

	// Add any nested errors
	for _, cause := range err.Causes {
		errors = append(errors, extractSchemaErrors(file, cause)...)
	}

	return errors
}

// validateExtraRules applies additional validation rules beyond JSON schema
func (v *Validator) validateExtraRules(sloWithFiles []SLOWithFile) []ValidationError {
	var errors []ValidationError

	// Check for duplicate IDs
	idSeen := make(map[string]string)
	for _, sloWithFile := range sloWithFiles {
		id := sloWithFile.SLO.Metadata.ID
		if prevFile, exists := idSeen[id]; exists {
			errors = append(errors, ValidationError{
				File:    sloWithFile.File,
				Path:    "metadata.id",
				Message: fmt.Sprintf("duplicate ID %q (also in %s)", id, filepath.Base(prevFile)),
			})
		} else {
			idSeen[id] = sloWithFile.File
		}

		// Check compliance window >= max burn policy window
		complianceErrors := validateComplianceWindow(sloWithFile.File, sloWithFile.SLO)
		errors = append(errors, complianceErrors...)
	}

	return errors
}

// validateComplianceWindow checks that compliance window >= max of all burn policy windows
func validateComplianceWindow(file string, slo *SLO) []ValidationError {
	var errors []ValidationError

	complianceDur, err := ParseDuration(slo.Spec.ComplianceWindow)
	if err != nil {
		errors = append(errors, ValidationError{
			File:    file,
			Path:    "spec.complianceWindow",
			Message: fmt.Sprintf("invalid duration: %v", err),
		})
		return errors
	}

	maxPolicyWindow := complianceDur
	for i, rule := range slo.Spec.BurnPolicy.Rules {
		shortDur, err := ParseDuration(rule.ShortWindow)
		if err != nil {
			errors = append(errors, ValidationError{
				File:    file,
				Path:    fmt.Sprintf("spec.burnPolicy.rules[%d].shortWindow", i),
				Message: fmt.Sprintf("invalid duration: %v", err),
			})
			continue
		}

		longDur, err := ParseDuration(rule.LongWindow)
		if err != nil {
			errors = append(errors, ValidationError{
				File:    file,
				Path:    fmt.Sprintf("spec.burnPolicy.rules[%d].longWindow", i),
				Message: fmt.Sprintf("invalid duration: %v", err),
			})
			continue
		}

		if shortDur > maxPolicyWindow {
			maxPolicyWindow = shortDur
		}
		if longDur > maxPolicyWindow {
			maxPolicyWindow = longDur
		}
	}

	if complianceDur < maxPolicyWindow {
		errors = append(errors, ValidationError{
			File: file,
			Path: "spec.complianceWindow",
			Message: fmt.Sprintf("complianceWindow (%s) must be >= max burn policy window (%s)",
				slo.Spec.ComplianceWindow, formatDuration(maxPolicyWindow)),
		})
	}

	return errors
}

// formatDuration converts a time.Duration back to a duration string
func formatDuration(d time.Duration) string {
	if d%(24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", d/(24*time.Hour))
	}
	if d%time.Hour == 0 {
		return fmt.Sprintf("%dh", d/time.Hour)
	}
	if d%time.Minute == 0 {
		return fmt.Sprintf("%dm", d/time.Minute)
	}
	return fmt.Sprintf("%ds", d/time.Second)
}
