// SPDX-License-Identifier: MPL-2.0

package linkprobe

// SetNextSeqForTest sets the next seq of the open stream, to reach 2^32 - 1 without 4e9 probes.
func (p *Prober) SetNextSeqForTest(n uint32) { p.nextSeq = uint64(n) }

// ProberForTest returns the prober of h.
func (h *Host) ProberForTest() *Prober { return h.prober }
