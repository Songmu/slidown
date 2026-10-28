package cmd

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"testing/synctest"

	"github.com/fswatcher/fswatcher"
	"github.com/spf13/cobra"
)

func TestWatchApplyLoopFileEvents(t *testing.T) {
	for _, tt := range []struct {
		name       string
		eventNames []string
		wantQueued int
	}{
		{name: "unrelated file", eventNames: []string{"/slides/other.md"}},
		{name: "matching file", eventNames: []string{"/slides/deck.md"}, wantQueued: 1},
		{
			name:       "full trigger buffer drops further matches",
			eventNames: []string{"/slides/other.md", "/slides/deck.md", "/slides/deck.md"},
			wantQueued: 1,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			events := make(chan fswatcher.Event, len(tt.eventNames))
			for _, name := range tt.eventNames {
				events <- fswatcher.Event{Name: name, Op: fswatcher.Write}
			}
			close(events)
			triggers := make(chan struct{}, 1)
			err := watchApplyLoop(context.Background(), &cobra.Command{}, "/slides/deck.md",
				events, nil, triggers, nil, func(context.Context) error {
					t.Fatal("file events must be debounced before applying")
					return nil
				})
			if err != nil {
				t.Fatalf("watchApplyLoop: %v", err)
			}
			if got := len(triggers); got != tt.wantQueued {
				t.Fatalf("queued triggers = %d, want %d", got, tt.wantQueued)
			}
			// The caller, not the loop, owns the trigger channel.
			close(triggers)
		})
	}
}

func TestWatchApplyLoopContinuesAfterApplyFailure(t *testing.T) {
	debounced := make(chan struct{}, 3)
	for range cap(debounced) {
		debounced <- struct{}{}
	}
	close(debounced)
	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&stderr)
	ctx := context.Background()
	calls := 0
	err := watchApplyLoop(ctx, cmd, "", nil, nil, nil, debounced, func(runCtx context.Context) error {
		if runCtx != ctx {
			t.Error("apply did not receive the loop context")
		}
		calls++
		if calls == 2 {
			return nil
		}
		return fmt.Errorf("apply failure %d", calls)
	})
	if err != nil {
		t.Fatalf("watchApplyLoop: %v", err)
	}
	if calls != 3 {
		t.Fatalf("apply calls = %d, want 3", calls)
	}
	if got, want := stderr.String(), "apply failure 1\napply failure 3\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestWatchApplyLoopWatcherErrors(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errorsCh := make(chan error)
		debounced := make(chan struct{})
		triggers := make(chan struct{}, 1)
		var stderr bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetErr(&stderr)
		calls := 0
		done := make(chan error, 1)
		go func() {
			done <- watchApplyLoop(ctx, cmd, "", nil, errorsCh, triggers, debounced,
				func(context.Context) error {
					calls++
					return nil
				})
		}()
		errorsCh <- fmt.Errorf("watcher failure")
		synctest.Wait()
		if got, want := stderr.String(), "watch error: watcher failure\n"; got != want {
			t.Fatalf("stderr = %q, want %q", got, want)
		}

		close(errorsCh)
		// A closed error channel must be disabled so the loop becomes idle.
		synctest.Wait()
		debounced <- struct{}{}
		synctest.Wait()
		if calls != 1 {
			t.Fatalf("apply calls after closing watcher errors = %d, want 1", calls)
		}
		cancel()
		if err := <-done; err != nil {
			t.Fatalf("watchApplyLoop: %v", err)
		}
		close(triggers)
	})
}

func TestWatchApplyLoopStops(t *testing.T) {
	for _, reason := range []string{"events closed", "debounced closed", "context canceled"} {
		t.Run(reason, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			events := make(chan fswatcher.Event)
			debounced := make(chan struct{})
			switch reason {
			case "events closed":
				close(events)
			case "debounced closed":
				close(debounced)
			case "context canceled":
				cancel()
			}
			err := watchApplyLoop(ctx, &cobra.Command{}, "", events, nil, nil, debounced,
				func(context.Context) error {
					t.Fatal("apply must not run while stopping")
					return nil
				})
			if err != nil {
				t.Fatalf("watchApplyLoop: %v", err)
			}
		})
	}
}

func TestWatchApplyLoopCancellationDuringApply(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		events := make(chan fswatcher.Event)
		debounced := make(chan struct{})
		started := make(chan struct{})
		var stderr bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetErr(&stderr)
		done := make(chan error, 1)
		go func() {
			done <- watchApplyLoop(ctx, cmd, "/slides/deck.md", events, nil, nil, debounced,
				func(runCtx context.Context) error {
					close(started)
					<-runCtx.Done()
					return fmt.Errorf("apply interrupted: %w", runCtx.Err())
				})
		}()
		debounced <- struct{}{}
		<-started
		select {
		case debounced <- struct{}{}:
			t.Fatal("loop must not receive another debounce while applying")
		default:
		}
		select {
		case events <- fswatcher.Event{Name: "/slides/deck.md"}:
			t.Fatal("loop must not receive events while applying")
		default:
		}
		cancel()
		if err := <-done; err != nil {
			t.Fatalf("watchApplyLoop: %v", err)
		}
		if got := stderr.String(); got != "" {
			t.Fatalf("cancellation during apply must not log an error, got %q", got)
		}
	})
}
