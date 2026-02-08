package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/samijaber1/aegis-slo/internal/slo"
)

func main() {
	validateCmd := flag.NewFlagSet("validate", flag.ExitOnError)
	validateDir := validateCmd.String("dir", "", "directory containing SLO YAML files")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "validate":
		validateCmd.Parse(os.Args[2:])
		if *validateDir == "" {
			fmt.Fprintln(os.Stderr, "Error: --dir flag is required")
			validateCmd.Usage()
			os.Exit(1)
		}
		os.Exit(runValidate(*validateDir))
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: aegis <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  validate --dir <path>    Validate SLO YAML files in a directory")
	fmt.Println()
}

func runValidate(dirPath string) int {
	// Find schema file relative to the binary or in the current directory
	schemaPath := findSchemaFile()
	if schemaPath == "" {
		fmt.Fprintln(os.Stderr, "Error: could not find schemas/slo_v1.json")
		return 1
	}

	// Create validator
	validator, err := slo.NewValidator(schemaPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to initialize validator: %v\n", err)
		return 1
	}

	// Validate directory
	errors := validator.ValidateDirectory(dirPath)

	if len(errors) == 0 {
		fmt.Println("✓ All SLO files are valid")
		return 0
	}

	// Group errors by file
	errorsByFile := make(map[string][]slo.ValidationError)
	for _, err := range errors {
		errorsByFile[err.File] = append(errorsByFile[err.File], err)
	}

	// Print errors grouped by file
	var files []string
	for file := range errorsByFile {
		files = append(files, file)
	}
	sort.Strings(files)

	fmt.Fprintf(os.Stderr, "✗ Validation failed with %d error(s):\n\n", len(errors))
	for _, file := range files {
		fileErrors := errorsByFile[file]
		for _, err := range fileErrors {
			if err.Path != "" {
				fmt.Fprintf(os.Stderr, "%s: %s: %s\n", filepath.Base(err.File), err.Path, err.Message)
			} else {
				fmt.Fprintf(os.Stderr, "%s: %s\n", filepath.Base(err.File), err.Message)
			}
		}
	}

	return 1
}

// findSchemaFile looks for the schema file in common locations
func findSchemaFile() string {
	// Try relative to current directory
	candidates := []string{
		"schemas/slo_v1.json",
		"../schemas/slo_v1.json",
		"../../schemas/slo_v1.json",
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}
