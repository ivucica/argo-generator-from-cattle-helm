package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// HelmChartSpec defines the desired state of HelmChart
type HelmChartSpec struct {
	Chart        string `yaml:"chart"`
	Version      string `yaml:"version"`
	Repo         string `yaml:"repo"`
	ValuesContent string `yaml:"valuesContent"`
}

// HelmChart is the Schema for the helmcharts API
type HelmChart struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec HelmChartSpec `yaml:"spec"`
}

// Output is the structure ArgoCD expects from the generator
type Output struct {
	Chart         string `json:"chart"`
	Version       string `json:"version"`
	RepoURL       string `json:"repoURL"`
	Values        string `json:"values"`
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
}

func main() {
	// ArgoCD provides the input path as the first argument
	if len(os.Args) != 2 {
		log.Fatal("Usage: helmchart-generator <path>")
	}
	inputPath := os.Args[1]

	outputList := []Output{}

	err := filepath.Walk(inputPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || (filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml") {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			log.Printf("Error opening file %s: %v", path, err)
			return nil // Continue with next file
		}
		defer file.Close()

		decoder := yaml.NewDecoder(file)
		for {
			var chart HelmChart
			if err := decoder.Decode(&chart); err != nil {
				if err == io.EOF {
					break
				}
				log.Printf("Error decoding YAML from %s: %v", path, err)
				continue // Continue to next document in the file
			}

			// Check if it's the correct Kind and APIVersion
			if chart.APIVersion == "helm.cattle.io/v1" && chart.Kind == "HelmChart" {
				output := Output{
					Chart:     chart.Spec.Chart,
					Version:   chart.Spec.Version,
					RepoURL:   chart.Spec.Repo,
					Values:    chart.Spec.ValuesContent,
					Name:      chart.Metadata.Name,
					Namespace: chart.Metadata.Namespace,
				}
				outputList = append(outputList, output)
			}
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error walking the path %s: %v", inputPath, err)
	}

	// The final output to ArgoCD must be a map with a single "items" key
	result := map[string][]Output{
		"items": outputList,
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		log.Fatalf("Error marshalling JSON: %v", err)
	}

	fmt.Println(string(jsonBytes))
}
