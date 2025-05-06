package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var tools = map[string]string{
	"helm": "tools/helm/latest/helm-v3.14.3-linux-amd64.tar.gz",
	"k3s":  "tools/k3s/latest/k3s",
	"flux": "tools/flux/latest/flux",
}

func main() {
	baseURL := flag.String("url", "", "Base URL of ProGet (e.g., https://proget.example.com/upack/privatsky-tools/download)")
	tool := flag.String("tool", "", "Tool to fetch: helm | k3s | flux")
	install := flag.Bool("install", false, "If true, place the binary in /usr/local/bin")

	flag.Parse()

	apiKey := os.Getenv("PROGET_API_KEY")
	if apiKey == "" {
		log.Fatal("PROGET_API_KEY environment variable is not set")
	}
	if *baseURL == "" || *tool == "" {
		log.Fatal("You must provide both --url and --tool")
	}

	relPath, ok := tools[*tool]
	if !ok {
		log.Fatalf("Unsupported tool: %s", *tool)
	}

	fullURL := fmt.Sprintf("%s/%s", strings.TrimRight(*baseURL, "/"), relPath)
	fmt.Printf("Fetching %s from %s\n", *tool, fullURL)

	resp, err := fetchWithAuth(fullURL, apiKey)
	if err != nil {
		log.Fatalf("Download failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		log.Fatalf("Artifact not found at: %s\nCheck if the tool is uploaded or if the folder structure (like Windows folders) interfered with packaging.", fullURL)
	}

	if *tool == "helm" {
		if err := extractTarGz(resp.Body, "linux-amd64/helm"); err != nil {
			log.Fatalf("Failed to extract helm binary: %v", err)
		}
		if *install {
			moveBinary("helm")
		}
	} else {
		outputFile := *tool
		out, err := os.Create(outputFile)
		if err != nil {
			log.Fatalf("Error creating file: %v", err)
		}
		defer out.Close()
		if _, err := io.Copy(out, resp.Body); err != nil {
			log.Fatalf("Error writing file: %v", err)
		}
		if err := os.Chmod(outputFile, 0755); err != nil {
			log.Fatalf("Error making %s executable: %v", outputFile, err)
		}
		if *install {
			moveBinary(outputFile)
		}
	}
}

func fetchWithAuth(url, apiKey string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth("api", apiKey)
	client := &http.Client{}
	return client.Do(req)
}

func extractTarGz(reader io.Reader, target string) error {
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return err
	}
	defer gz.Close()

	tarReader := tar.NewReader(gz)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		if header.Typeflag == tar.TypeReg && header.Name == target {
			outFile, err := os.Create("helm")
			if err != nil {
				return err
			}
			defer outFile.Close()
			if _, err := io.Copy(outFile, tarReader); err != nil {
				return err
			}
			return os.Chmod("helm", 0755)
		}
	}
	return fmt.Errorf("binary %s not found in archive", target)
}

func moveBinary(name string) {
	dst := filepath.Join("/usr/local/bin", name)
	if err := exec.Command("mv", name, dst).Run(); err != nil {
		log.Fatalf("Failed to move binary to /usr/local/bin: %v", err)
	}
	fmt.Printf("Installed %s to /usr/local/bin\n", name)
}
