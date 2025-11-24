package floodwait

import (
	"context"
	"fmt"
	"io"
	"time"
)

const tickInterval = 100 * time.Millisecond

// Start begins a countdown that keeps updating the same line with the remaining
// delay. It stops automatically when the timer finishes or the context is
// cancelled.
func Start(ctx context.Context, out io.Writer, delay time.Duration, render func(time.Duration) string) {
	if out == nil || delay <= 0 || render == nil {
		return
	}

	go func() {
		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()

		deadline := time.Now().Add(delay)

		for {
			remaining := time.Until(deadline)
			if remaining < 0 {
				remaining = 0
			}

			fmt.Fprint(out, render(remaining))

			if remaining <= 0 {
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
