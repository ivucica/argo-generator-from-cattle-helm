package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/src-d/go-git.v4" // A popular Go Git library
	"gopkg.in/yaml.v3"
)

// --- Structs for ArgoCD Plugin Communication ---

type PluginRequest struct {
	ApplicationSetName string `json:"applicationSetName"`
	Input              struct {
		Parameters map[string]string `json:"parameters"`
	} `json:"input"`
}

type PluginResponse struct {
	Output struct {
		Parameters []map[string]string `json:"parameters"`
	} `json:"output"`
}

// --- Structs for parsing Rancher HelmCharts ---

type HelmChartSpec struct {
	Chart         string `yaml:"chart"`
	Version       string `yaml:"version"`
	Repo          string `yaml:"repo"`
	ValuesContent string `yaml:"valuesContent"`
}

type HelmChart struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec HelmChartSpec `yaml:"spec"`
}

var pluginToken string

func main() {
	// The ApplicationSet controller writes the token to this path.
	tokenBytes, err := os.ReadFile("/var/run/argo/token")
	if err != nil {
		log.Fatalf("Failed to read token: %v", err)
	}
	pluginToken = strings.TrimSpace(string(tokenBytes))

	http.HandleFunc("/api/v1/getparams.execute", handleGetParams)
	log.Println("Starting HelmChart plugin server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleGetParams(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
		return
	}

	// 1. Authenticate the request from the ApplicationSet controller
	authHeader := r.Header.Get("Authorization")
	expectedHeader := "Bearer " + pluginToken
	if authHeader != expectedHeader {
		log.Printf("Forbidden: Invalid token. Expected '%s', got '%s'", expectedHeader, authHeader)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 2. Decode the incoming request
	var req PluginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Bad request: %v", err), http.StatusBadRequest)
		return
	}

	// 3. Get repo details from the input parameters
	repoURL, ok1 := req.Input.Parameters["repoURL"]
	revision, ok2 := req.Input.Parameters["revision"]
	path, ok3 := req.Input.Parameters["path"]
	if !ok1 || !ok2 || !ok3 {
		http.Error(w, "Bad request: repoURL, revision, and path are required parameters", http.StatusBadRequest)
		return
	}

	// 4. Clone the Git repository to a temporary directory
	tempDir, err := os.MkdirTemp("", "argo-plugin-")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create temp dir: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	_, err = git.PlainClone(tempDir, false, &git.CloneOptions{
		URL:           repoURL,
		ReferenceName: plumbing.NewBranchReferenceName(revision), // Or use plumbing.NewTagReferenceName for tags
		SingleBranch:  true,
		Depth:         1,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to clone repo: %v", err), http.StatusInternalServerError)
		return
	}

	// 5. Walk the cloned repo and find HelmChart files
	var generatedParams []map[string]string
	searchPath := filepath.Join(tempDir, path)

	err = filepath.Walk(searchPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || (filepath.Ext(filePath) != ".yaml" && filepath.Ext(filePath) != ".yml") {
			return nil
		}

		file, err := os.Open(filePath)
		if err != nil {
			return nil // Skip files we can't open
		}
		defer file.Close()

		decoder := yaml.NewDecoder(file)
		for {
			var chart HelmChart
			if err := decoder.Decode(&chart); err != nil {
				if err == io.EOF {
					break
				}
				continue
			}

			if chart.APIVersion == "helm.cattle.io/v1" && chart.Kind == "HelmChart" {
				params := map[string]string{
					"chart":     chart.Spec.Chart,
					"version":   chart.Spec.Version,
					"repoURL":   chart.Spec.Repo,
					"values":    chart.Spec.ValuesContent,
					"name":      chart.Metadata.Name,
					"namespace": chart.Metadata.Namespace,
				}
				generatedParams = append(generatedParams, params)
			}
		}
		return nil
	})

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to process repo: %v", err), http.StatusInternalServerError)
		return
	}

	// 6. Send the response back to the ApplicationSet controller
	resp := PluginResponse{}
	resp.Output.Parameters = generatedParams

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
