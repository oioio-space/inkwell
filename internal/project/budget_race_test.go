//go:build race

package project

import "time"

// listBudget is the time TXT-13 allows for listing 10000 files; the race
// detector slows system calls down by an order of magnitude.
const listBudget = 1000 * time.Millisecond
