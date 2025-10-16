package coverage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime/coverage"
	"time"
)

var hostnameHeader = "x-hostname"
var filenameHeader = "x-filename"

// requestOtherProfile fetches coverage profile data from other pods (instances) in the cluster.
// It writes each response to a subdirectory in parentDir, avoiding duplicates based on hostname.
func requestOtherProfile(parentDir, profileURL string) error {
	hosts := map[string]struct{}{hostname: {}} // Track hosts we've already received data from
	reqTimeout := 3 * time.Second              // Timeout for individual requests
	allReqTimeout := 15 * time.Second          // Total timeout for gathering all profiles
	now := time.Now()

	for len(hosts) < numberPods {
		// Stop if total timeout exceeded
		if time.Since(now) > allReqTimeout {
			return fmt.Errorf("requests timeout exceeded")
		}

		// Create HTTP request with context timeout
		ctx, cancel := context.WithTimeout(context.Background(), reqTimeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, profileURL, http.NoBody)
		if err != nil {
			cancel()
			return err
		}

		req.Header.Set(hostnameHeader, hostname)

		// Execute the request
		resp, err := http.DefaultClient.Do(req)
		cancel()
		if err != nil {
			return fmt.Errorf("failed to perform request: %w", err)
		}

		// Read response headers
		responseHostname := resp.Header.Get(hostnameHeader)
		filename := resp.Header.Get(filenameHeader)

		// Skip if we already received data from this host
		if _, ok := hosts[responseHostname]; ok {
			continue
		}
		hosts[responseHostname] = struct{}{}

		// Read the response body (binary coverage data)
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		// Write the received coverage data to a file
		err = write(body, parentDir, responseHostname, filename)
		if err != nil {
			return fmt.Errorf("failed to write response to file: %w", err)
		}

		resp.Body.Close()
	}

	return nil
}

// write saves coverage data to a temporary directory under parentDir.
// It also writes the coverage metadata using runtime/coverage.
func write(p []byte, parentDir, dirName, fileName string) error {
	tmpCov, _ := os.MkdirTemp(parentDir, dirName)

	// Write coverage metadata
	if err := coverage.WriteMetaDir(tmpCov); err != nil {
		return fmt.Errorf("error writing meta coverage data: %v", err)
	}

	// Create and write the actual coverage file
	f, err := os.Create(filepath.Join(tmpCov, fileName))
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer f.Close()

	_, err = f.Write(p)
	if err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}

	return nil
}

// resetOtherProfile sends requests to all other pods to reset their coverage counters.
// It avoids sending duplicate requests to the same hostname.
func resetOtherProfile(resetURL string) error {
	hosts := map[string]struct{}{hostname: {}} // Track hosts we've already reset
	reqTimeout := 3 * time.Second              // Timeout for individual requests
	allReqTimeout := 15 * time.Second          // Total timeout for resetting all pods
	now := time.Now()

	for len(hosts) < numberPods {
		// Stop if total timeout exceeded
		if time.Since(now) > allReqTimeout {
			return fmt.Errorf("requests timeout exceeded")
		}

		// Create HTTP request with context timeout
		ctx, cancel := context.WithTimeout(context.Background(), reqTimeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, resetURL, http.NoBody)
		if err != nil {
			cancel()
			return err
		}

		req.Header.Set(hostnameHeader, hostname)

		// Execute the request
		resp, err := http.DefaultClient.Do(req)
		cancel()
		if err != nil {
			return fmt.Errorf("failed to perform request: %w", err)
		}

		responseHostname := resp.Header.Get(hostnameHeader)

		// Skip if we've already reset this host
		if _, ok := hosts[responseHostname]; ok {
			continue
		}
		hosts[responseHostname] = struct{}{}

		resp.Body.Close()
	}

	return nil
}
