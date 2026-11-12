//go:build !server

package app

import (
	"context"

	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

// withLinkStepLog does nothing: only the headless end-to-end build records the steps of the link
// sampler.
func withLinkStepLog(context.Context, *p2p.Options) {}
