//go:build darwin

package memory

func sysTotalMemory() uint64 {
	n, err := sysctlUint64("hw.memsize")
	if err != nil {
		return 0
	}
	return n
}

// Free memory is unavailable rather than invoking the external vm_stat tool.
// Inkwell's libp2p resource manager only uses TotalMemory.
func sysFreeMemory() uint64 { return 0 }
