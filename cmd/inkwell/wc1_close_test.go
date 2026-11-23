package main

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

type closeFixture struct {
	coordinator *closeCoordinator
	frames      map[uint]chan closeFrame
	generations map[uint]uint64
	released    chan uint
}

func newCloseFixture(t *testing.T, windows ...uint) *closeFixture {
	t.Helper()
	f := &closeFixture{
		coordinator: newCloseCoordinator(),
		frames:      make(map[uint]chan closeFrame),
		generations: make(map[uint]uint64),
		released:    make(chan uint, 8),
	}
	for _, id := range windows {
		f.coordinator.register(id, func() { f.released <- id })
		f.connect(t, id)
	}
	return f
}

func (f *closeFixture) connect(t *testing.T, window uint) {
	t.Helper()
	frames := make(chan closeFrame, 8)
	f.frames[window] = frames
	send := func(frame closeFrame) error {
		frames <- frame
		return nil
	}
	f.generations[window] = f.coordinator.attach(window, send, func(frame closeFrame) {
		frames <- frame
	})
	if !f.coordinator.ready(window, f.generations[window]) || (<-frames).Kind != "ready" {
		t.Fatalf("fixture failed to start editor %d", window)
	}
}

func (f *closeFixture) frame(t *testing.T, window uint, want string) closeFrame {
	t.Helper()
	synctest.Wait()
	select {
	case frame := <-f.frames[window]:
		if frame.Kind != want || frame.Attempt == 0 {
			t.Fatalf("window %d got %+v, want %s with an attempt ID", window, frame, want)
		}
		return frame
	default:
		t.Fatalf("window %d did not receive %s", window, want)
		return closeFrame{}
	}
}

func (f *closeFixture) reply(window uint, attempt uint32, kind string) {
	f.coordinator.report(window, f.generations[window], closeFrame{Kind: kind, Attempt: attempt})
}

func (f *closeFixture) confirm(window uint, attempt uint32) {
	f.reply(window, attempt, "saved")
	f.coordinator.presence(window, attempt)
	f.reply(window, attempt, "complete")
}

func (f *closeFixture) releasedWindow(t *testing.T, window uint) {
	t.Helper()
	synctest.Wait()
	select {
	case got := <-f.released:
		if got != window {
			t.Fatalf("released window %d, want %d", got, window)
		}
	default:
		t.Fatalf("window %d was not released", window)
	}
}

func (f *closeFixture) held(t *testing.T) {
	t.Helper()
	synctest.Wait()
	if len(f.released) != 0 {
		t.Fatalf("window %d was released without matching authorization", <-f.released)
	}
}

func TestWC1W1CloseHold(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newCloseFixture(t, 1, 2)
		first, repeated := f.coordinator.requestWindow(1), f.coordinator.requestWindow(1)
		if first || repeated {
			t.Fatal("W-1: close without approval passed the hook")
		}
		a := f.frame(t, 1, "prepare")
		if len(f.frames[1]) != 0 {
			t.Fatal("W-1: repeated close started a second attempt")
		}
		f.confirm(1, a.Attempt)
		f.releasedWindow(t, 1)
		if !f.coordinator.requestWindow(1) || f.coordinator.requestWindow(1) {
			t.Fatal("W-1: OS re-request did not consume the one-use permit")
		}
		f.held(t)
	})
	t.Run("replace before consumption", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newCloseFixture(t, 1)
			if f.coordinator.requestWindow(1) {
				t.Fatal("close without permit")
			}
			a := f.frame(t, 1, "prepare")
			f.confirm(1, a.Attempt)
			f.releasedWindow(t, 1)
			f.connect(t, 1)
			if f.coordinator.requestWindow(1) {
				t.Fatal("W-1: permit survived page replacement")
			}
		})
	})
}

func TestWC1W2SaveAck(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newCloseFixture(t, 1, 2)
		f.coordinator.requestWindow(1)
		a := f.frame(t, 1, "prepare")
		f.confirm(2, a.Attempt)
		f.coordinator.report(1, f.generations[1]+1, closeFrame{Kind: "saved", Attempt: a.Attempt})
		f.reply(1, a.Attempt, "complete")
		f.held(t)
		f.reply(1, a.Attempt, "saved")
		f.reply(1, a.Attempt, "complete")
		f.held(t)
		f.coordinator.presence(1, a.Attempt)
		f.reply(1, a.Attempt, "complete")
		f.releasedWindow(t, 1)
	})
}

func TestWC1W4ExitPaths(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newCloseFixture(t, 1, 2)
		if !f.coordinator.requestWindow(3) || f.coordinator.shouldQuit() {
			t.Fatal("W-4: a presentation required a save or application quit bypassed editors")
		}
		a := f.frame(t, 1, "prepare")
		b := f.frame(t, 2, "prepare")
		if a.Attempt != b.Attempt {
			t.Fatal("W-4: quit did not have one shared attempt")
		}
		f.confirm(1, a.Attempt)
		f.held(t)
		time.Sleep(5 * time.Second)
		f.frame(t, 1, "abort")
		f.frame(t, 2, "abort")
		f.confirm(2, b.Attempt)
		f.held(t)
	})
	t.Run("replacement after the first close of a group", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newCloseFixture(t, 1, 2)
			f.coordinator.requestAll()
			a := f.frame(t, 1, "prepare")
			f.frame(t, 2, "prepare")
			f.confirm(1, a.Attempt)
			f.confirm(2, a.Attempt)
			synctest.Wait()
			if !f.coordinator.requestWindow(1) || f.coordinator.editorClosed(1) {
				t.Fatal("W-4: the first editor did not close independently")
			}
			f.connect(t, 2)
			if f.coordinator.requestWindow(2) {
				t.Fatal("W-4: the second editor's old permit survived replacement")
			}
			b := f.frame(t, 2, "prepare")
			f.confirm(2, b.Attempt)
			if !f.coordinator.requestWindow(2) {
				t.Fatal("W-4: the second editor could not close after a fresh cut")
			}
		})
	})
	t.Run("close before a page can edit", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newCloseFixture(t)
			f.coordinator.register(1, func() { f.released <- 1 })
			g := f.coordinator.attach(1, func(closeFrame) error { return nil }, func(closeFrame) {})
			f.coordinator.register(2, func() { f.released <- 2 })
			f.coordinator.drain = func(until time.Time) { time.Sleep(time.Until(until)) }
			if g == 0 {
				t.Fatal("W-4: the test stream did not attach")
			}
			f.coordinator.requestAll()
			if f.coordinator.ready(1, g) {
				t.Fatal("W-4: startup was enabled after the window was counted as empty")
			}
			synctest.Wait()
			if len(f.released) != 2 ||
				!f.coordinator.requestWindow(1) || !f.coordinator.requestWindow(2) {
				t.Fatal("W-4: the never-ready windows did not close immediately")
			}
		})
	})
	t.Run("a missing stream after editing still needs a cut", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newCloseFixture(t, 1)
			f.coordinator.detach(1, f.generations[1])
			f.coordinator.requestWindow(1)
			f.held(t)
			time.Sleep(closeDeadline)
			f.held(t)
			if a := f.coordinator.current; a == nil || a.failure.Reason != "timeout" {
				t.Fatal("W-4: a disconnected editor that could have edits needs recovery")
			}
		})
	})
	t.Run("a live page can confirm it has not started", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newCloseFixture(t, 1, 2)
			f.coordinator.requestAll()
			a := f.frame(t, 1, "prepare")
			f.frame(t, 2, "prepare")
			f.reply(1, a.Attempt, "empty")
			f.held(t)
			f.confirm(2, a.Attempt)
			synctest.Wait()
			if !f.coordinator.requestWindow(1) || !f.coordinator.requestWindow(2) {
				t.Fatal("W-4: an empty cut blocked a saved editor's close")
			}
		})
	})
	t.Run("refused take-over clears the close view", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newCloseFixture(t, 1)
			paused := false
			f.coordinator.pause = func(pause bool) { paused = pause }
			f.reply(1, 0, "project")
			a := f.frame(t, 1, "prepare")
			f.confirm(1, a.Attempt)
			f.frame(t, 1, "project-approved")
			if !f.coordinator.beginProjectChange() {
				t.Fatal("W-4: the saved take-over was not allowed to begin")
			}
			f.coordinator.rejectProjectChange()
			f.frame(t, 1, "abort")
			if paused || f.coordinator.current != nil {
				t.Fatal("W-4: the refused take-over left a save-failure notice")
			}
			f.held(t)
			f.coordinator.requestWindow(1)
			f.frame(t, 1, "prepare")
		})
	})
	t.Run("Linux logout waits for both editors", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newCloseFixture(t, 1, 2)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			answer := make(chan bool, 1)
			go func() { answer <- f.coordinator.waitExit(ctx) }()
			a := f.frame(t, 1, "prepare")
			f.frame(t, 2, "prepare")
			f.confirm(2, a.Attempt)
			f.held(t)
			if len(answer) != 0 {
				t.Fatal("W-4: session manager was answered before both cuts")
			}
			f.confirm(1, a.Attempt)
			if !<-answer || !f.coordinator.requestWindow(1) || !f.coordinator.requestWindow(2) || !f.coordinator.shouldQuit() {
				t.Fatal("W-4: approved group could not finish logout")
			}
		})
	})
	t.Run("PC1 approval precedes the irreversible drain", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newCloseFixture(t, 1)
			if f.coordinator.beginProjectChange() {
				t.Fatal("W-4: project change lacks a durable cut")
			}
			f.reply(1, 0, "project")
			a := f.frame(t, 1, "prepare")
			f.confirm(1, a.Attempt)
			f.frame(t, 1, "project-approved")
			if !f.coordinator.beginProjectChange() || f.coordinator.beginProjectChange() {
				t.Fatal("W-4: project change approval is not one use")
			}
			f.held(t)
			if f.coordinator.requestWindow(1) {
				t.Fatal("W-4: native window closed before recording the project change")
			}
			f.coordinator.finishProjectChange()
			f.releasedWindow(t, 1)
			if !f.coordinator.requestWindow(1) {
				t.Fatal("W-4: registered project change could not close")
			}
		})
	})
}

func TestWC1W6CloseRecovery(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newCloseFixture(t, 1)
		f.coordinator.requestWindow(1)
		a := f.frame(t, 1, "prepare")
		f.reply(1, a.Attempt, "failed")
		f.frame(t, 1, "abort")
		f.held(t)
		f.coordinator.dismiss(a.Attempt)
		f.coordinator.force(a.Attempt)
		f.held(t)
		f.coordinator.requestWindow(1)
		b := f.frame(t, 1, "prepare")
		f.reply(1, b.Attempt, "failed")
		f.frame(t, 1, "abort")
		f.coordinator.retry(b.Attempt)
		c := f.frame(t, 1, "prepare")
		if c.Attempt == a.Attempt || c.Attempt == b.Attempt {
			t.Fatal("W-6: retry reused an attempt")
		}
		f.confirm(1, b.Attempt)
		f.held(t)
		f.coordinator.detach(1, f.generations[1])
		f.coordinator.force(c.Attempt)
		f.releasedWindow(t, 1)
		if !f.coordinator.requestWindow(1) || f.coordinator.requestWindow(1) {
			t.Fatal("W-6: force did not confer exactly one close permit")
		}
	})
	t.Run("force gives up only on renderers that did not confirm", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newCloseFixture(t, 1, 2)
			f.coordinator.requestAll()
			a := f.frame(t, 1, "prepare")
			f.frame(t, 2, "prepare")
			f.confirm(2, a.Attempt) // window 1's page never answers
			time.Sleep(closeDeadline)
			f.frame(t, 1, "abort")
			f.frame(t, 2, "abort")
			f.coordinator.force(a.Attempt)
			synctest.Wait()
			if len(f.released) != 2 {
				t.Fatalf("W-6: force released %d of 2 windows", len(f.released))
			}
			if !f.coordinator.requestWindow(1) || !f.coordinator.requestWindow(2) {
				t.Fatal("W-6: force did not confer the close permits")
			}
			if !f.coordinator.abandoned(1) || f.coordinator.abandoned(2) {
				t.Fatalf("W-6: abandoned renderers = %v, %v; want only the silent window's",
					f.coordinator.abandoned(1), f.coordinator.abandoned(2))
			}
		})
		synctest.Test(t, func(t *testing.T) {
			f := newCloseFixture(t, 1)
			f.coordinator.requestWindow(1)
			a := f.frame(t, 1, "prepare")
			f.confirm(1, a.Attempt)
			f.releasedWindow(t, 1)
			if !f.coordinator.requestWindow(1) || f.coordinator.abandoned(1) {
				t.Fatal("W-6: a confirmed close gave up on its renderer")
			}
		})
	})
	t.Run("abort while the stream is full", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newCloseFixture(t, 1)
			editor := f.coordinator.editors[1]
			send := editor.send
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			capacity := make(chan struct{})
			editor.abort = queueCloseAbort(ctx, func(frame closeFrame) error {
				<-capacity
				return send(frame)
			})
			editor.send = func(frame closeFrame) error {
				if frame.Kind == "abort" {
					return errors.New("stream queue full")
				}
				return send(frame)
			}
			f.coordinator.requestWindow(1)
			a := f.frame(t, 1, "prepare")
			f.reply(1, a.Attempt, "failed")
			f.coordinator.dismiss(a.Attempt)
			f.held(t)
			close(capacity)
			if abort := f.frame(t, 1, "abort"); abort.Attempt != a.Attempt {
				t.Fatal("W-6: cooperative page did not receive the abort for its hold")
			}
			f.held(t)
		})
	})
	for _, tc := range []struct {
		name      string
		knownLost bool
	}{
		{name: "renderer lost before close", knownLost: true},
		{name: "renderer lost during close"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := newCloseFixture(t, 1)
				c := f.coordinator
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				views := make(chan closeView, 8)
				go c.watchView(ctx, 1, func(view closeView) { views <- view })
				synctest.Wait()
				if tc.knownLost {
					c.rendererCrashed(1)
				}
				if c.requestWindow(1) {
					t.Fatal("W-6: close was allowed without consent")
				}
				if !tc.knownLost {
					f.frame(t, 1, "prepare")
					if progress := <-views; progress.attempt == 0 || progress.failure.Kind != "" {
						t.Fatalf("W-6: expected saving view before loss, got %+v", progress)
					}
					time.Sleep(time.Second)
				}
				lostAt := time.Now()
				if !tc.knownLost {
					c.rendererCrashed(1)
				}
				synctest.Wait()
				var notice closeView
				select {
				case notice = <-views:
				default:
					t.Fatal("W-6: recovery was not shown immediately after losing the renderer")
				}
				if notice.attempt == 0 || notice.failure.Reason != "crashed" || !time.Now().Equal(lostAt) {
					t.Fatalf("W-6: expected immediate crash recovery, got %+v after %s", notice, time.Since(lostAt))
				}
				if tc.knownLost && c.current.timer != nil {
					t.Fatal("W-6: close armed a wait for a renderer already known lost")
				}
				if c.requestWindow(1) {
					t.Fatal("W-6: repeated close was allowed without consent")
				}
				f.held(t)
				if len(views) != 0 || c.current.id != notice.attempt {
					t.Fatal("W-6: repeated close replaced or stacked the recovery dialog")
				}
				c.force(notice.attempt)
				f.releasedWindow(t, 1)
				if !c.requestWindow(1) || c.requestWindow(1) {
					t.Fatal("W-6: force did not confer exactly one close permit")
				}
			})
		})
	}
}
