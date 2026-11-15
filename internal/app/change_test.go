package app

import (
	"context"
	"errors"
	"os"
	"regexp"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/trust"
)

// marker is a change write that only leaves a file in the member-list directory, so that a test
// sees whether, and when, it ran.
func marker(ran *bool) changeWrite {
	return func(st *opStore) error {
		*ran = true
		if err := st.ensureOpen(); err != nil {
			return err
		}
		return st.root.WriteFile("change-marker", []byte("written"), 0o600)
	}
}

// foreignOp returns the genesis op of another device's project: an op this store does not hold.
func foreignOp(t *testing.T) trust.Op {
	t.Helper()
	proj, _ := newProjectFolder(t)
	_, st, err := openTrust(proj, newDevice(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.close() }()
	ops, err := st.readAll()
	if err != nil || len(ops) == 0 {
		t.Fatalf("reading the other project's ops: %v, %d", err, len(ops))
	}
	return ops[0]
}

// TestRequestsRefusedOnceTakeOverRequested: from a change request on, the requests that would
// change or sign under the old project (Invite, InviteDevice, Join, Ask) and a second change
// request are refused: one change per process image (rule N-13). The take-over (N-5) and the
// join (J-1) enter through this request.
func TestRequestsRefusedOnceTakeOverRequested(t *testing.T) {
	rt, _ := startRuntime(t, nil, false)
	s := rt.Session
	if _, err := s.Invite("writer"); err != nil {
		t.Fatalf("Invite before the request: %v", err)
	}
	var ran bool
	if err := s.requestChange(t.Context(), marker(&ran)); err != nil {
		t.Fatalf("change request: %v", err)
	}
	if !rt.RestartRequested() {
		t.Error("N-12 a request restarts: no restart recorded")
	}
	var second bool
	if err := s.requestChange(t.Context(), marker(&second)); !errors.Is(err, errLeaving) {
		t.Errorf("N-13 one change per image: a second change request = %v, want refused", err)
	}
	refused := map[string]error{}
	_, refused["Invite"] = s.Invite("writer")
	_, refused["InviteDevice"] = s.InviteDevice()
	_, refused["Join"] = s.Join(t.Context(), "inkwell:not-a-ticket")
	_, refused["Ask"] = s.Ask(t.Context(), "peer", "Ana")
	for name, err := range refused {
		if !errors.Is(err, errLeaving) {
			t.Errorf("N-13 one change per image: %s after the request = %v, want refused", name, err)
		}
	}
	if err := rt.Close(); err != nil {
		t.Fatal(err)
	}
	if !ran || second {
		t.Errorf("N-13 one change per image: first write ran %v, second write ran %v; want true, false", ran, second)
	}
}

// TestChangeRequestFailsWhenDrainTimesOut: a bridge handler that never returns makes the request
// fail at the drain's deadline: nothing is recorded, nothing is written when the runtime closes,
// and no restart follows (rule N-16).
func TestChangeRequestFailsWhenDrainTimesOut(t *testing.T) {
	rt, d := startRuntime(t, nil, false)
	folder := d.proj.Path()
	if !rt.Bridge.Begin() {
		t.Fatal("Begin refused before any request")
	}
	t.Cleanup(rt.Bridge.End)
	before := trustFiles(t, folder)
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	var ran bool
	if err := rt.Session.requestChange(ctx, marker(&ran)); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("change request with a stuck handler = %v, want the drain's deadline", err)
	}
	if err := rt.Close(); err != nil {
		t.Fatal(err)
	}
	if ran || !equalMaps(trustFiles(t, folder), before) {
		t.Error("N-16 no write before the drain: the member list was written although the drain failed")
	}
	if rt.RestartRequested() {
		t.Error("N-16 no write before the drain: a failed request still restarts")
	}
}

// TestChangeRequestLosesToClose: a request still draining when the runtime closes (the person
// quit meanwhile) records nothing afterwards: it fails, nothing is written and no restart follows;
// it never restarts on a change that was not written (rules N-12, N-16).
func TestChangeRequestLosesToClose(t *testing.T) {
	rt, d := startRuntime(t, nil, false)
	folder := d.proj.Path()
	if !rt.Bridge.Begin() {
		t.Fatal("Begin refused before any request")
	}
	before := trustFiles(t, folder)
	var ran bool
	res := make(chan error, 1)
	go func() { res <- rt.Session.requestChange(t.Context(), marker(&ran)) }()
	waitUntil(t, "the request is draining", func() bool { return rt.Session.leaving() != nil })
	if err := rt.Close(); err != nil {
		t.Fatal(err)
	}
	rt.Bridge.End() // the handler in flight returns: the drain completes after the close
	if err := <-res; !errors.Is(err, errClosing) {
		t.Errorf("N-12 no restart without its write: a request that lost to the close = %v, want refused", err)
	}
	if ran || rt.RestartRequested() || !equalMaps(trustFiles(t, folder), before) {
		t.Errorf("N-12 no restart without its write: write ran %v, restart recorded %v", ran, rt.RestartRequested())
	}
}

// TestTrustStoreUsableAfterWiringClose: the change write runs after the wiring is closed, and the
// member-list store is still open then: an op saved by the write is read back, and is on disk
// once the runtime is closed (rule N-13).
func TestTrustStoreUsableAfterWiringClose(t *testing.T) {
	rt, d := startRuntime(t, nil, false)
	op := foreignOp(t)
	var wiringClosed bool
	write := func(st *opStore) error {
		_, err := rt.wiring.Issue(trust.OpAdd, newDevice(t).Public(), trust.RoleWriter, "late")
		wiringClosed = err != nil
		if err := st.save(op); err != nil {
			return err
		}
		ops, err := st.readAll()
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(ops, func(o trust.Op) bool { return o.Hash() == op.Hash() }) {
			return errors.New("saved op not read back")
		}
		return nil
	}
	if err := rt.Session.requestChange(t.Context(), write); err != nil {
		t.Fatal(err)
	}
	if err := rt.Close(); err != nil {
		t.Fatalf("N-13 trust store usable after Wiring.Close: Close = %v", err)
	}
	if !wiringClosed {
		t.Error("N-13 nothing writes once the write starts: the wiring still issued an op during the write")
	}
	if _, ok := trustFiles(t, d.proj.Path())[op.Hash().String()+opExt]; !ok {
		t.Error("N-13 trust store usable after Wiring.Close: the op saved by the write is not on disk")
	}
}

// countingSigner counts the signatures made with a device's key.
type countingSigner struct {
	trust.Signer
	n atomic.Int64
}

func (c *countingSigner) Sign(domain string, msg []byte) []byte {
	c.n.Add(1)
	return c.Signer.Sign(domain, msg)
}

// TestNoTrustWriteNorSignatureOnceTheWriteStarts: when the change write starts, every other
// source of a member-list write or of a signature has ended. A late op from each source is
// refused inside the write: a peer's list sync, an attestation of the page's heads, an admission
// by the lobby's host, a pending join. The device's key signs nothing from then on, and the member
// list on disk is the one of before the request until the write itself (rule N-13).
func TestNoTrustWriteNorSignatureOnceTheWriteStarts(t *testing.T) {
	rt, d := startRuntime(t, nil, false)
	folder := d.proj.Path()
	signer := &countingSigner{}
	if err := rt.wiring.do(func() { signer.Signer, rt.wiring.signer = rt.wiring.signer, signer }); err != nil {
		t.Fatal(err)
	}
	// The key signs while the session runs: the counter sees what the wiring signs.
	if err := rt.wiring.AttestHeads("doc", []trust.Hash{{1}}); err != nil || signer.n.Load() == 0 {
		t.Fatalf("setup: an attestation before the request = %v, %d signatures; want one signed", err, signer.n.Load())
	}
	before := trustFiles(t, folder)
	lenBefore := rt.wiring.list().Len()
	late := foreignOp(t)
	var problems []string
	signedAtWrite := int64(-1)
	write := func(*opStore) error {
		signedAtWrite = signer.n.Load()
		syncMsg := transport.Event{Kind: transport.Message, Peer: "late", Payload: trust.Encode(late)}
		if err := rt.wiring.do(func() { rt.wiring.onTrust(syncMsg) }); !errors.Is(err, transport.ErrClosed) {
			problems = append(problems, "list sync: a peer's message is still taken: "+errString(err))
		}
		if err := rt.wiring.AttestHeads("doc", []trust.Hash{{2}}); !errors.Is(err, transport.ErrClosed) {
			problems = append(problems, "attestation: heads are still signed: "+errString(err))
		}
		// The lobby's host admits through this call ([invites.newHost], Issue).
		guest := trust.Op{Kind: trust.OpAdd, Subject: newDevice(t).Public(), Role: trust.RoleWriter, Name: "late"}
		if _, err := rt.inv.wiring.IssueWith(guest); !errors.Is(err, transport.ErrClosed) {
			problems = append(problems, "lobby: the host still admits: "+errString(err))
		}
		if rt.inv.ctx.Err() == nil {
			problems = append(problems, "lobby: still open")
		}
		if _, err := rt.inv.join(t.Context(), "inkwell:not-a-ticket"); !errors.Is(err, errLeaving) {
			problems = append(problems, "pending join: still taken: "+errString(err))
		}
		if got := trustFiles(t, folder); !equalMaps(got, before) || rt.wiring.list().Len() != lenBefore {
			problems = append(problems, "the member list changed before the write")
		}
		return nil
	}
	if err := rt.Session.requestChange(t.Context(), write); err != nil {
		t.Fatal(err)
	}
	if err := rt.Close(); err != nil {
		t.Fatal(err)
	}
	if signedAtWrite < 0 || len(problems) > 0 {
		t.Errorf("N-13 nothing writes once the write starts: write ran %v, %v", signedAtWrite >= 0, problems)
	}
	if n := signer.n.Load() - signedAtWrite; signedAtWrite >= 0 && n != 0 {
		t.Errorf("N-13 nothing writes once the write starts: the device's key signed %d times after the write started", n)
	}
}

func errString(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}

// TestStartupNoticeShownOnce: an image started by the restart of a take-over says once, from the
// standing it actually loaded, how it went: « done » for a member, « failed » otherwise; an image
// started otherwise says nothing. Every image has its own 128-bit startup id (rule N-12).
func TestStartupNoticeShownOnce(t *testing.T) {
	foreign, _, _ := foreignFolder(t)
	own, _ := newProjectFolder(t)
	for _, tc := range []struct {
		name  string
		after bool
		want  string
	}{
		{"member after a take-over", true, "foreign.takeOver.done"},
		{"not a member after a take-over", true, "foreign.takeOver.failed"},
		{"ordinary start", false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proj := own
			if tc.want == "foreign.takeOver.failed" {
				proj = foreign
			}
			home, err := os.OpenRoot(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = home.Close() })
			rt, err := Start(t.Context(), home, Options{Project: proj, RendezvousURL: NoCodes, LANService: testLANService, AfterChange: tc.after})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = rt.Close() })
			if got := rt.Session.StartupNotice(); got != tc.want {
				t.Errorf("N-12 notice from the loaded standing: first StartupNotice = %q, want %q", got, tc.want)
			}
			if got := rt.Session.StartupNotice(); got != "" {
				t.Errorf("N-12 notice shown once: second StartupNotice = %q, want none", got)
			}
			if id := rt.Session.StartupID(); !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(id) {
				t.Errorf("startup id %q is not 128 random bits in hex", id)
			}
		})
	}
}
