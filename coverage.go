// Package coverage provides HTTP handlers for collecting and exposing
// code coverage data during runtime.
//
// To use this package, simply import it in your program:
//
//	import _ "github.com/compashka/coverage"
//
// It automatically registers several endpoints under /debug/coverage/:
//
//	/debug/coverage/          - returns current coverage percentage
//	/debug/coverage/reset     - resets collected coverage data
//	/debug/coverage/html      - shows HTML representation of coverage
//	/debug/coverage/profile   - returns binary coverage profile
//
// If your application does not already run an HTTP server, you must start one.
// For example:
//
//	go func() {
//		log.Println(http.ListenAndServe("localhost:6060", nil))
//	}()
//
// By default, all the handlers listed above are registered with the
// [http.DefaultServeMux]. If you are using a custom mux, you need to
// register them manually using:
//
//	mux.HandleFunc("/debug/coverage/", coverage.CovPercentHandler)
//	mux.HandleFunc("/debug/coverage/reset", coverage.CovResetHandler)
//	mux.HandleFunc("/debug/coverage/html", coverage.CovHTMLHandler)
//	mux.HandleFunc("/debug/coverage/profile", coverage.CovBinProfileHandler)
//
// The package is designed to work in multi-instance deployments (for example,
// multiple pods in Kubernetes). Use SetNumberPods to tell the package how many
// instances (pods) should be queried/aggregated when computing overall coverage.
// The numberPods value controls how the collection logic iterates over all
// instances to gather coverage data from each of them.

package coverage

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
)

// init registers the HTTP handlers for coverage endpoints
// and initializes the default logger.
func init() {
	http.HandleFunc("/debug/coverage/", CovPercentHandler)
	http.HandleFunc("/debug/coverage/reset", CovResetHandler)
	http.HandleFunc("/debug/coverage/html", CovHTMLHandler)
	http.HandleFunc("/debug/coverage/profile", CovBinProfileHandler)

	covLogger = newDefaultLogger()
}

var numberPods = 1

// SetNumberPods sets the number of pods (instances) that contribute to the
// coverage statistics.
//
// This value should reflect the number of running instances serving your
// application (for example the number of pods in a Kubernetes Deployment).
// The collection logic will iterate over this many targets when attempting to
// fetch coverage information from all instances and aggregate a global view.
//
// Example:
//
//	// tell the coverage package there are 5 pods to poll
//	coverage.SetNumberPods(5)
func SetNumberPods(n int) {
	numberPods = n
}

// hostname provides a unique identifier for the current instance.
// If os.Hostname() fails, it generates a random fallback name.
var hostname = func() string {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Sprintf("generated-hostname-%d", rand.Int())
	}
	return hostname
}()

var covLogger logger

// SetLogger allows setting a custom logger for coverage-related events.
func SetLogger(l logger) {
	covLogger = l
}
