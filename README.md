# Coverage Profiling for Go Applications

`coverage-profiling` is a Go package that provides HTTP handlers and utilities to collect, merge, and display **runtime code coverage** data in multi-instance Go applications. It is particularly useful in environments like **Kubernetes** where multiple pods run the same application and you want to aggregate coverage metrics across all instances.

---

## Why use this package?

- Collect coverage data during runtime, not just during unit tests.
- Aggregate coverage metrics across multiple instances (pods) to get a global view.
- Generate HTML coverage reports for easier visualization.
- Reset coverage counters on demand, either locally or across all instances.
- Useful for **integration tests**, end-to-end testing, and CI/CD pipelines where multiple services run simultaneously.

---

## Features

- Automatic HTTP endpoints for coverage collection:
    - `/debug/coverage/` — returns the current average coverage percentage.
    - `/debug/coverage/reset` — resets all coverage counters.
    - `/debug/coverage/html` — generates an HTML coverage report.
    - `/debug/coverage/profile` — returns binary coverage profile.
- Multi-instance aggregation using `SetNumberPods(n int)`.
- Custom logging via `SetLogger(logger)`.
- Works without modifying your existing code — just import the package.

---

## Installation

```bash
go get github.com/compashka/coverage-profiling/coverage
```

## Usage

```go
package main

import _ "github.com/compashka/coverage-profiling"
```
