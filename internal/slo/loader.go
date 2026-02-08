package slo

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadFromDirectory discovers and loads all SLO files from a directory
func LoadFromDirectory(dirPath string) ([]SLOWithFile, []ValidationError) {
	var slos []SLOWithFile
	var errors []ValidationError

	// Discover YAML files
	files, err := discoverYAMLFiles(dirPath)
	if err != nil {
		errors = append(errors, ValidationError{
			File:    dirPath,
			Message: fmt.Sprintf("failed to read directory: %v", err),
		})
		return nil, errors
	}

	// Parse each file
	for _, file := range files {
		slo, err := parseYAMLFile(file)
		if err != nil {
			errors = append(errors, ValidationError{
				File:    file,
				Message: fmt.Sprintf("failed to parse YAML: %v", err),
			})
			continue
		}
		slos = append(slos, SLOWithFile{
			SLO:  slo,
			File: file,
		})
	}

	return slos, errors
}

// discoverYAMLFiles finds all *.yaml and *.yml files in a directory
func discoverYAMLFiles(dirPath string) ([]string, error) {
	var files []string

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

// parseYAMLFile parses a single YAML file into an SLO struct
func parseYAMLFile(filePath string) (*SLO, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var slo SLO
	if err := yaml.Unmarshal(data, &slo); err != nil {
		return nil, err
	}

	return &slo, nil
}
