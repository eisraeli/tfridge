package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/urfave/cli/v2"
)

const appVersion = "0.0.1"

type ModuleInfo struct {
	Versions    []string `json:"versions"`
	Description string   `json:"description"`
	Source      string   `json:"source"`
}

type ProviderInfo struct {
	Versions []string `json:"versions"`
}

func main() {
	rootPath := createNewCliApp()

	moduleMap := make(map[string]string)
	providerMap := make(map[string]string)

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories starting with "."
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
			return filepath.SkipDir
		}

		// Process only .tf files
		if !info.IsDir() && filepath.Ext(path) == ".tf" {
			if err := extractModules(path, moduleMap, providerMap); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Print all unique modules found with their current and latest versions
	for source, currentVersion := range moduleMap {
		latestVersion, err := getLatestVersion(source)
		if err != nil {
			fmt.Printf("Error fetching latest version for %s: %s\n", source, err)
			continue
		}

		fmt.Printf("Module source: %s\n", source)
		fmt.Printf("Current version: %s\n", currentVersion)
		if latestVersion == "" {
			fmt.Printf("Latest version: Not found\n\n")
		} else {
			fmt.Printf("Latest version: %s\n\n", latestVersion)
		}
	}

	// Print all unique providers found with their current and latest versions
	for source, currentVersion := range providerMap {
		latestVersion, err := getLatestProviderVersion(source)
		if err != nil {
			fmt.Printf("Error fetching latest version for provider %s: %s\n", source, err)
			continue
		}

		fmt.Printf("Provider source: %s\n", source)
		fmt.Printf("Current version: %s\n", currentVersion)
		if latestVersion == "" {
			fmt.Printf("Latest version: Not found\n\n")
		} else {
			fmt.Printf("Latest version: %s\n\n", latestVersion)
		}
	}
}

// extractModules scans a Terraform file and extracts module sources and versions
func extractModules(filePath string, moduleMap, providerMap map[string]string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	moduleRegex := regexp.MustCompile(`module\s+"[^"]+"\s*{`)

	// Regular expressions to extract source and version
	sourceRegex := regexp.MustCompile(`source\s*=\s*["']([^"']+)["']`)
	versionRegex := regexp.MustCompile(`version\s*=\s*["']([^"']+)["']`)
	providerRegex := regexp.MustCompile(`provider\s*["']([^"']+)["']`)

	for scanner.Scan() {
		line := scanner.Text()
		if moduleRegex.MatchString(line) {
			source := ""
			version := ""

			for scanner.Scan() {
				line = scanner.Text()
				if sourceMatch := sourceRegex.FindStringSubmatch(line); sourceMatch != nil {
					source = sourceMatch[1]
				}
				if versionMatch := versionRegex.FindStringSubmatch(line); versionMatch != nil {
					version = versionMatch[1]
				}
				if line == "}" {
					break
				}
			}

			if source != "" {
				moduleMap[source] = version
			}
		} else if providerRegex.MatchString(line) {
			provider := ""
			version := ""

			if providerMatch := providerRegex.FindStringSubmatch(line); providerMatch != nil {
				provider = providerMatch[1]
			}
			if versionMatch := versionRegex.FindStringSubmatch(line); versionMatch != nil {
				version = versionMatch[1]
			}

			if provider != "" {
				providerMap[provider] = version
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func getLatestVersion(moduleSource string) (string, error) {
	parts := strings.Split(moduleSource, "//")
	module := parts[0]

	url := fmt.Sprintf("https://registry.terraform.io/v1/modules/%s", module)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch latest version, status code: %d", resp.StatusCode)
	}

	var moduleInfo ModuleInfo
	if err := json.NewDecoder(resp.Body).Decode(&moduleInfo); err != nil {
		return "", err
	}

	if len(moduleInfo.Versions) == 0 {
		return "Not found", nil
	}

	var validVersions []*semver.Version
	for _, v := range moduleInfo.Versions {
		if version, err := semver.NewVersion(v); err == nil {
			validVersions = append(validVersions, version)
		}
	}

	sort.Slice(validVersions, func(i, j int) bool {
		return validVersions[i].GreaterThan(validVersions[j])
	})

	return validVersions[0].String(), nil
}

func getLatestProviderVersion(providerSource string) (string, error) {
	// Check if the provider name already contains a namespace
	parts := strings.Split(providerSource, "/")
	if len(parts) == 2 {
		// This is already in the correct format (namespace/provider)
	} else if len(parts) == 1 {
		// Assume it is a HashiCorp provider without the namespace
		providerSource = "hashicorp/" + providerSource
	} else {
		return "", fmt.Errorf("provider format is incorrect: %s", providerSource)
	}

	// Construct the URL for the provider registry
	url := fmt.Sprintf("https://registry.terraform.io/v1/providers/%s", providerSource)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch latest version for provider, status code: %d", resp.StatusCode)
	}

	var providerInfo ProviderInfo
	if err := json.NewDecoder(resp.Body).Decode(&providerInfo); err != nil {
		return "", err
	}

	if len(providerInfo.Versions) == 0 {
		return "Not found", nil
	}

	var validVersions []*semver.Version
	for _, v := range providerInfo.Versions {
		if version, err := semver.NewVersion(v); err == nil {
			validVersions = append(validVersions, version)
		}
	}

	sort.Slice(validVersions, func(i, j int) bool {
		return validVersions[i].GreaterThan(validVersions[j])
	})

	return validVersions[0].String(), nil
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}

func createNewCliApp() string {
	var rootPath string

	app := &cli.App{
		Name:    "TFridge",
		Usage:   "Scan a specified directory for Terraform module and provider updates",
		Version: appVersion,

		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return cli.Exit("Please specify a path to the directory you want to scan", 1)
			}

			rootPath = c.Args().Get(0) // Modify the outer rootPath variable

			if !pathExists(rootPath) {
				errMsg := fmt.Sprintf("Path '%s' does not exist.", rootPath)
				return cli.Exit(errMsg, 1)
			}

			fmt.Println("Scanning directory:", rootPath)
			fmt.Println("")

			return nil
		},
	}

	// Run the app
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

	return rootPath
}
