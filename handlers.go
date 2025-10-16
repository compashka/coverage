// Package coverage provides HTTP handlers and utility functions
// to collect, merge, and display Go runtime coverage data across multiple pods.
package coverage

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/coverage"
	"strconv"
	"strings"
)

// writeBinCoverage writes current process coverage metadata and counters
// into a temporary directory and returns its path.
// It uses runtime/coverage package to dump the binary coverage data.
func writeBinCoverage() (string, error) {
	// Create a temporary directory for coverage data
	tmpDir, err := os.MkdirTemp("", "coverage")
	if err != nil {
		return "", fmt.Errorf("error creating temporary directory: %v", err)
	}

	tmpCov, err := os.MkdirTemp(tmpDir, hostname)
	if err != nil {
		return "", fmt.Errorf("error creating temporary directory: %v", err)
	}

	if err := coverage.WriteMetaDir(tmpCov); err != nil {
		return "", fmt.Errorf("error writing meta coverage data: %v", err)
	}

	if err := coverage.WriteCountersDir(tmpCov); err != nil {
		return "", fmt.Errorf("error writing coverage data: %v", err)
	}

	return tmpDir, nil
}

// mergeProfiles merges multiple coverage directories into one
// using the "go tool covdata merge" command.
func mergeProfiles(tmpDir string) (string, error) {
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return "", err
	}

	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(tmpDir, e.Name()))
		}
	}

	mergedDir, err := os.MkdirTemp(tmpDir, "merged")
	if err != nil {
		return "", err
	}

	cmd := exec.Command("go", "tool", "covdata", "merge",
		"-i="+strings.Join(dirs, ","),
		"-o="+mergedDir,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("error running merging coverage data: %v\n%s", err, string(output))
	}

	return mergedDir, nil
}

// getURL constructs a URL for the local coverage endpoints.
// If isReset is true, it returns the URL for the reset handler,
// otherwise for the profile handler.
func getURL(r *http.Request, isReset bool) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	path := "/debug/coverage/profile"
	if isReset {
		path = "/debug/coverage/reset"
	}

	return fmt.Sprintf("%s://%s%s", scheme, r.Host, path)
}

// CovPercentHandler collects coverage data from all pods,
// merges it, calculates average coverage percentage, and returns it as plain text.
func CovPercentHandler(w http.ResponseWriter, r *http.Request) {
	tmpDir, err := writeBinCoverage()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		covLogger.Errorf(err.Error())
	}

	if err := requestOtherProfile(tmpDir, getURL(r, false)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		covLogger.Errorf(err.Error())
	}

	mergedDir, err := mergeProfiles(tmpDir)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		covLogger.Errorf(err.Error())
	}

	cmd := exec.Command("go", "tool", "covdata", "percent", "-i="+mergedDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		covLogger.Errorf(fmt.Sprintf("error running merging coverage data: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	totalCoverage, err := calculateTotalCoverage(string(output))
	if err != nil {
		covLogger.Errorf(fmt.Sprintf("error calculating total coverage data: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("Total Average Coverage: %.2f%%\n", totalCoverage)))
}

// calculateTotalCoverage parses the textual output of "go tool covdata percent"
// and computes the average coverage across all listed packages.
func calculateTotalCoverage(output string) (float64, error) {
	var totalCoverage float64
	var packageCount int

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "coverage:") {
			parts := strings.Split(line, "coverage:")
			if len(parts) != 2 {
				continue
			}

			// Extract coverage value
			coveragePart := strings.TrimSpace(parts[1])
			coverageValue := strings.Split(coveragePart, "%")[0]
			coverageValue = strings.TrimSpace(coverageValue)

			coverage, err := strconv.ParseFloat(coverageValue, 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse coverage value: %v", err)
			}

			packages := strings.Split(parts[0], "\t")
			for _, pkg := range packages {
				pkg = strings.TrimSpace(pkg)
				if pkg != "" {
					packageCount++
				}
			}

			totalCoverage += coverage
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error reading output: %v", err)
	}

	if packageCount == 0 {
		return 0, fmt.Errorf("no coverage data found in the output")
	}

	averageCoverage := totalCoverage / float64(packageCount)
	return averageCoverage, nil
}

// CovResetHandler resets all coverage counters locally and across all pods.
func CovResetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(hostnameHeader) == hostname {
		w.WriteHeader(http.StatusOK)
		return
	}

	err := coverage.ClearCounters()
	if err != nil {
		covLogger.Errorf(fmt.Sprintf("error clearing counters: %v", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err = resetOtherProfile(getURL(r, true)); err != nil {
		covLogger.Errorf(fmt.Sprintf("error resetting other profiles: %v", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write([]byte("Coverage counters have been reset"))
}

// CovHTMLHandler generates an HTML coverage report
// by merging coverage data from all pods and converting it to an HTML file.
func CovHTMLHandler(w http.ResponseWriter, r *http.Request) {
	tmpDir, err := writeBinCoverage()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		covLogger.Errorf(err.Error())
		return
	}

	if err := requestOtherProfile(tmpDir, getURL(r, false)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		covLogger.Errorf(err.Error())
		return
	}

	mergedDir, err := mergeProfiles(tmpDir)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		covLogger.Errorf(err.Error())
		return
	}

	// Convert binary coverage data to text format
	textOutputFile := filepath.Join(mergedDir, "coverage.out")
	cmd := exec.Command("go", "tool", "covdata", "textfmt", "-i="+mergedDir, "-o="+textOutputFile)
	if _, err = cmd.CombinedOutput(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		covLogger.Errorf(err.Error())
		return
	}

	// Generate HTML coverage report
	htmlOutputFile := filepath.Join(mergedDir, "coverage.html")
	cmd = exec.Command("go", "tool", "cover", "-html="+textOutputFile, "-o="+htmlOutputFile)
	covLogger.Infof("generate HTML coverage report")

	if _, err = cmd.CombinedOutput(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		covLogger.Errorf(err.Error())
		return
	}

	// Read and return the generated HTML file
	f, err := os.ReadFile(htmlOutputFile)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		covLogger.Errorf(err.Error())
		return
	}

	w.Write(f)
}

// CovBinProfileHandler writes binary coverage data to the HTTP response
// so it can be fetched by other pods in a cluster.
func CovBinProfileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(hostnameHeader) == hostname {
		w.WriteHeader(http.StatusOK)
		return
	}

	tmpDir, err := os.MkdirTemp("", "coverage")
	if err != nil {
		covLogger.Errorf(fmt.Sprintf("error creating temporary directory: %v", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	// Write metadata files
	if err := coverage.WriteMetaDir(tmpDir); err != nil {
		covLogger.Errorf(fmt.Sprintf("error writing meta data: %v", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Write counter files
	if err := coverage.WriteCountersDir(tmpDir); err != nil {
		covLogger.Errorf(fmt.Sprintf("error writing counters data: %v", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Printf("Convert the binary coverage data to text format\n")

	files, err := os.ReadDir(tmpDir)
	if err != nil {
		covLogger.Errorf(fmt.Sprintf("error reading temporary directory: %v", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, f := range files {
		if !strings.HasPrefix(f.Name(), "covcounters") {
			continue
		}

		filePath := filepath.Join(tmpDir, f.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("Failed to read file %s: %v\n", f.Name(), err)
			continue
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set(filenameHeader, f.Name())
		w.Header().Set(hostnameHeader, hostname)
		w.Write(data)
	}
}
