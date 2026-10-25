//go:build !race

package project

import "time"

// listBudget is the time TXT-13 allows for listing 10000 files in a test.
// The target is 50 ms (BenchmarkList measures about 14 ms); the test allows
// five times that so that it still catches a quadratic walk when `go test`
// runs many packages at once on a busy machine.
const listBudget = 250 * time.Millisecond
