package sidecar

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// The tier probe's bounds (ADR 0044 point 2): each probe launch is bounded by
// probeTimeout; an inconclusive answer is retried after a backoff.
const (
	probeTimeout    = 5 * time.Second
	probeBackoffMin = time.Second
	probeBackoffMax = 30 * time.Second
)

// handshakeTimeout bounds each side of the launch handshake: the parent's wait
// for the tier report and the launcher's wait for « proceed » (ADR 0044
// point 3, plan T-4).
const handshakeTimeout = 5 * time.Second

// Launcher exit statuses the parent reads (a target never runs before the
// handshake, so they cannot be the target's).
const (
	// probeUnsupportedExit: a namespace or mount step was refused (ADR 0044 point 2).
	probeUnsupportedExit = 120
	// seccompUnavailableExit: the seccomp filter could not be installed.
	seccompUnavailableExit = 121
)

var (
	errNamespaceRefused   = errors.New("sidecar: the namespace launch was refused")
	errSeccompUnavailable = errors.New("sidecar: mandatory seccomp unavailable")
)

// launchPlan is what a launch runs: the program's descriptor and the tier.
type launchPlan struct {
	prog program
	tier tier
}

// isolated: tier N has a private network namespace.
func (p launchPlan) isolated() bool { return p.tier == tierN }

// mounted: a bounded output folder is a private tmpfs in tier N only.
func (p launchPlan) mounted() bool { return p.tier == tierN }

// programFor is the descriptor of the executable spec runs: its base name,
// which [Store.Resolve] gives as the manifest name.
func programFor(path string) (program, error) {
	name := filepath.Base(path)
	p, ok := lookupProgram(name)
	if !ok {
		return program{}, fmt.Errorf("%w: no sandbox descriptor for %q", ErrUnsupported, name)
	}
	return p, nil
}

// planLaunch decides the tier of one launch (ADR 0044 point 2), once per
// process for the machine's facts, per launch for the program.
func planLaunch(spec Spec) (launchPlan, error) {
	p, err := programFor(spec.Path)
	if err != nil {
		return launchPlan{}, err
	}
	if spec.tier != tierNone {
		if spec.Protected && spec.tier == tierF {
			return launchPlan{}, fmt.Errorf("%w: %s", ErrProtectedRefused, p.name)
		}
		return launchPlan{prog: p, tier: spec.tier}, nil
	}
	if err := unproven(p); err != nil {
		return launchPlan{}, err
	}
	facts, err := sandboxFacts(spec.Store)
	if err != nil {
		return launchPlan{}, err
	}
	facts.Protected = spec.Protected
	t, err := decideTier(p, facts)
	return launchPlan{prog: p, tier: t}, err
}

// hostArch is the architecture the tier decision is made for: GOARCH, a
// variable so that tests can decide for another one.
var hostArch = runtime.GOARCH

// unproven is the definitive refusal of p on an architecture whose inventory
// has not passed (plan S-7), nil otherwise. It comes before any probe: there
// the probe's own target may be refused a system call, and an inconclusive
// probe would hide this refusal and its sentence.
func unproven(p program) error {
	if slices.Contains(p.proven, hostArch) {
		return nil
	}
	_, err := decideTier(p, tierFacts{Arch: hostArch})
	return err
}

// sandboxFacts gathers the facts once: conclusive facts are kept for good,
// inconclusive ones are retried after a backoff (ADR 0044 point 2).
var sandboxFacts = cachedFacts(probeFacts, time.Now)

// cachedFacts serialises probe calls. A conclusive answer is cached; an
// inconclusive one is returned as ErrProbeInconclusive until a backoff
// measured on now has elapsed. The probe's target is verified in store before
// each probe; a target that cannot be verified is that error, neither a fact
// nor a retry (a build without its sidecars is not a slow machine).
func cachedFacts(probe func(target string) tierFacts, now func() time.Time) func(store *Store) (tierFacts, error) {
	var mu sync.Mutex
	var decided bool
	var facts tierFacts
	var lastErr error
	var retryAt time.Time
	var backoff time.Duration
	return func(store *Store) (tierFacts, error) {
		mu.Lock()
		defer mu.Unlock()
		if decided {
			return facts, nil
		}
		if lastErr != nil && now().Before(retryAt) {
			return tierFacts{}, lastErr
		}
		target, err := probeTarget(store)
		if err != nil {
			return tierFacts{}, fmt.Errorf("sidecar: tier probe target: %w", err)
		}
		f := probe(target)
		if _, err := decideTier(anyProgram(), f); errors.Is(err, ErrProbeInconclusive) {
			backoff = min(max(2*backoff, probeBackoffMin), probeBackoffMax)
			retryAt = now().Add(backoff)
			lastErr = err
			return tierFacts{}, err
		}
		decided, facts = true, f
		return f, nil
	}
}

// anyProgram is proven on this architecture: it tells whether facts are
// conclusive, which does not depend on the program.
func anyProgram() program { return program{name: "probe", proven: []string{hostArch}} }

// probeFacts runs the real N launch, then, if it was refused, the real F
// launch of the same target (ADR 0044 point 2). Each launch is bounded by
// probeTimeout.
func probeFacts(target string) tierFacts {
	f := tierFacts{Arch: hostArch, Landlock: landlockABI(), Seccomp: true}
	if f.Landlock < minLandlockABI {
		return f
	}
	err := probeLaunch(target, tierN)
	f.NS, f.NSErr = probeOutcome(tierN, err), err
	if errors.Is(err, errSeccompUnavailable) {
		f.Seccomp = false
		return f
	}
	if f.NS != launchRefused {
		return f
	}
	err = probeLaunch(target, tierF)
	f.F, f.FErr = probeOutcome(tierF, err), err
	if errors.Is(err, errSeccompUnavailable) {
		f.Seccomp = false
	}
	return f
}

// probeOutcome classes a probe launch: a namespace refusal (an errno of
// clone, the id maps, a mount, or the launcher's status 120) is « refused »;
// a timeout, a signal or an unknown failure is « inconclusive ». For F, any
// launcher failure is « failed » (refused), a timeout inconclusive.
func probeOutcome(t tier, err error) launchOutcome {
	switch {
	case err == nil:
		return launchOK
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled), errors.Is(err, errProbeSignalled):
		return launchInconclusive
	case errors.Is(err, errSeccompUnavailable):
		return launchRefused
	case t == tierF:
		return launchRefused
	case errors.Is(err, errNamespaceRefused):
		return launchRefused
	}
	return namespaceOutcome(err)
}

var errProbeSignalled = errors.New("sidecar: the probe launch was killed by a signal")

// probeLaunch is a real launch of target in tier t, with the strictest
// variant N uses: no network, output bounded (ADR 0044 point 2).
var probeLaunch = func(target string, t tier) error {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	return runProbe(ctx, target, t)
}

// probeTarget is the probe's target (ADR 0044 point 2, amended 2026-11-15):
// the pinned typst, verified in store as before every launch. It depends on no
// library of the host, unlike this executable, which the probe used to launch
// and which is linked to GTK in the application. A variable so that tests can
// probe another target.
var probeTarget = func(store *Store) (string, error) {
	return cmp.Or(store, defaultStore).Resolve(probeName)
}

// runProbe launches `target --version` as a typst launch is: typst's own
// descriptor, hence the real filter (the probe is the launch, plan T-3).
func runProbe(ctx context.Context, target string, t tier) error {
	prog, err := programFor(probeName)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "inkwell-tier-probe-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	out := filepath.Join(dir, "out")
	if err := os.Mkdir(out, 0o700); err != nil {
		return err
	}
	spec := Spec{
		Path: target, Args: []string{"--version"}, Dir: dir, Env: []string{},
		NoNetwork: true, WatchDir: out, WriteFiles: []string{"probe"}, MaxDirBytes: int64(os.Getpagesize()),
	}
	if err := validateLaunch(ctx, &spec); err != nil {
		return err
	}
	o, err := run(ctx, spec, launchPlan{prog: prog, tier: t}, newDirectoryMount)
	switch {
	case err != nil:
		return err
	case o.Signal != 0:
		return fmt.Errorf("%w: %d", errProbeSignalled, o.Signal)
	case o.ExitCode != 0:
		return launcherExit(o.ExitCode)
	}
	return nil
}

// launcherExit names a launcher's own failure status.
func launcherExit(code int) error {
	switch code {
	case probeUnsupportedExit:
		return fmt.Errorf("%w (status %d)", errNamespaceRefused, code)
	case seccompUnavailableExit:
		return fmt.Errorf("%w (status %d)", errSeccompUnavailable, code)
	}
	return fmt.Errorf("sidecar: the launcher failed with status %d", code)
}

// landlockABI is the kernel's Landlock ABI, 0 when unavailable.
func landlockABI() int {
	abi, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, unix.LANDLOCK_CREATE_RULESET_VERSION)
	if errno != 0 {
		return 0
	}
	return int(abi) // #nosec G115 -- a small ABI number.
}

// tierNamespaces gives cmd the namespaces of tier t: in N every one (user,
// mount, PID, IPC, network), the caller mapped to root inside so that the
// launcher can mount; in F none (ADR 0044 point 3: one configuration per tier).
func tierNamespaces(cmd *exec.Cmd, plan launchPlan) {
	if plan.tier != tierN {
		return
	}
	a := cmd.SysProcAttr
	a.Cloneflags |= syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS | syscall.CLONE_NEWPID | syscall.CLONE_NEWIPC | syscall.CLONE_NEWNET
	a.UidMappings = []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}}
	a.GidMappings = []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}}
	a.GidMappingsEnableSetgroups = false
}

// baseConfig is the launcher configuration every launch of plan shares.
func baseConfig(spec Spec, plan launchPlan) sandboxConfig {
	c := sandboxConfig{
		RequireDeath: spec.DeathTest != "",
		ReadPaths:    spec.ReadPaths,
		LandlockABI:  spec.landlockABI,
		WorkingDir:   spec.Dir,
		Memory:       spec.MemoryLimit,
		File:         spec.FileSizeLimit,
		Overlay:      plan.tier == tierN,
		Tier:         plan.tier.String(),
		Program:      plan.prog.name,
		Syscalls:     plan.prog.syscalls[runtime.GOARCH],
		Prctl:        plan.prog.prctl,
		Ioctl:        plan.prog.ioctl,
	}
	if plan.tier == tierF {
		c.ParentPID = os.Getpid()
		c.ArmDelay = spec.armDelay
	}
	return c
}

// handshakeEnv names the descriptor of the tier report; « proceed » is the next.
const handshakeEnv = "INKWELL_INTERNAL_HANDSHAKE_FD"

// handshake is the parent's side of the launch handshake.
type handshake struct {
	report, reportChild, proceed, proceedChild *os.File
	tier                                       tier
	// Test seams: afterReport runs between the verified report and
	// « proceed »; skipProceed never sends it.
	afterReport func()
	skipProceed bool
}

func newHandshake(cmd *exec.Cmd, plan launchPlan) (*handshake, error) {
	h := &handshake{tier: plan.tier}
	var err error
	if h.report, h.reportChild, err = os.Pipe(); err != nil {
		return nil, err
	}
	if h.proceedChild, h.proceed, err = os.Pipe(); err != nil {
		h.close()
		return nil, err
	}
	cmd.Env = append(cmd.Env, handshakeEnv+"="+strconv.Itoa(3+len(cmd.ExtraFiles)))
	cmd.ExtraFiles = append(cmd.ExtraFiles, h.reportChild, h.proceedChild)
	return h, nil
}

func (h *handshake) close() {
	for _, f := range []*os.File{h.report, h.reportChild, h.proceed, h.proceedChild} {
		if f != nil {
			_ = f.Close()
		}
	}
}

// tierReport is what the launcher established, written after every
// confinement step and before the target starts.
type tierReport struct {
	tier       string
	namespaces [5]string // user, mnt, pid, ipc, net
	ownProc    bool      // /proc is this PID namespace's
	seccomp    int       // Seccomp: of /proc/self/status, 2 for a filter
	landlock   int
}

var namespaceKinds = [5]string{"user", "mnt", "pid", "ipc", "net"}

func (r tierReport) String() string {
	return fmt.Sprintf("tier=%s ns=%s proc=%t seccomp=%d landlock=%d", r.tier, strings.Join(r.namespaces[:], ","), r.ownProc, r.seccomp, r.landlock)
}

// parseTierReport accepts exactly one report line, nothing else.
func parseTierReport(line string) (tierReport, error) {
	var r tierReport
	invalid := fmt.Errorf("sidecar: invalid tier report %q", line)
	body, ok := strings.CutSuffix(line, "\n")
	fields := strings.Fields(body)
	if !ok || len(fields) != 5 {
		return r, invalid
	}
	values := map[string]string{}
	for _, f := range fields {
		k, v, found := strings.Cut(f, "=")
		if !found || values[k] != "" || v == "" {
			return r, invalid
		}
		values[k] = v
	}
	r.tier = values["tier"]
	ns := strings.Split(values["ns"], ",")
	seccomp, ok1 := reportNumber(values["seccomp"], 2)     // the modes of /proc/self/status: 0, 1, 2
	landlock, ok2 := reportNumber(values["landlock"], 255) // a Landlock ABI number
	if len(ns) != len(namespaceKinds) || !ok1 || !ok2 || (values["proc"] != "true" && values["proc"] != "false") || (r.tier != "N" && r.tier != "F") {
		return r, invalid
	}
	for i, kind := range namespaceKinds {
		if !namespaceIdentity(kind, ns[i]) {
			return r, invalid
		}
	}
	r.seccomp, r.landlock = seccomp, landlock
	copy(r.namespaces[:], ns)
	r.ownProc = values["proc"] == "true"
	return r, nil
}

// reportNumber parses a canonical decimal (no sign, no leading zero) of at
// most limit.
func reportNumber(v string, limit uint64) (int, bool) {
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil || n > limit || strconv.FormatUint(n, 10) != v {
		return 0, false
	}
	return int(n), true // #nosec G115 -- n <= limit, a small bound.
}

// namespaceIdentity reports whether v is a namespace identity of kind as
// readlink(2) of /proc/<pid>/ns/<kind> gives it: `kind:[inode]`, the inode a
// canonical non-zero decimal.
func namespaceIdentity(kind, v string) bool {
	inode, ok := strings.CutPrefix(v, kind+":[")
	if !ok {
		return false
	}
	inode, ok = strings.CutSuffix(inode, "]")
	n, err := strconv.ParseUint(inode, 10, 64)
	return ok && err == nil && n != 0 && strconv.FormatUint(n, 10) == inode
}

// ownNamespaces are this process's namespace identities.
func ownNamespaces() ([5]string, error) {
	var ns [5]string
	for i, kind := range namespaceKinds {
		link, err := os.Readlink("/proc/self/ns/" + kind)
		if err != nil {
			return ns, err
		}
		ns[i] = link
	}
	return ns, nil
}

// verify compares the report with the decided tier: in N every namespace
// differs from the parent's, in F every one is the parent's; always a seccomp
// filter, Landlock ABI ≥ 3 and the process's own /proc.
func (h *handshake) verify(r tierReport) error {
	parent, err := ownNamespaces()
	if err != nil {
		return err
	}
	ok := r.tier == h.tier.String() && r.seccomp == 2 && r.landlock >= minLandlockABI && r.ownProc
	for i := range parent {
		if (r.namespaces[i] == parent[i]) != (h.tier == tierF) {
			ok = false
		}
	}
	if !ok {
		return fmt.Errorf("sidecar: the launch reports %s, decided tier %v", r, h.tier)
	}
	return nil
}

// await reads the tier report within the deadline and ctx, verifies it and
// lets the launcher proceed. Any failure is final: the caller kills the
// launcher, which never started the target.
func (h *handshake) await(ctx context.Context) error {
	_ = h.reportChild.Close()
	_ = h.proceedChild.Close()
	if err := h.report.SetReadDeadline(time.Now().Add(handshakeTimeout)); err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, func() { _ = h.report.SetReadDeadline(time.Now()) })
	defer stop()
	line, err := bufio.NewReader(h.report).ReadString('\n')
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("sidecar: no tier report: %w", err)
	}
	r, err := parseTierReport(line)
	if err != nil {
		return err
	}
	if err := h.verify(r); err != nil {
		return err
	}
	if h.afterReport != nil {
		h.afterReport()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if h.skipProceed {
		return nil
	}
	_, err = h.proceed.Write([]byte("p"))
	return err
}
