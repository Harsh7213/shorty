package mailqueue

import (
	"context"
	"log"
	"sync"
	"time"
)

// Start launches the outbox dispatcher and email worker in the background.
// Each one is supervised: if its RabbitMQ connection drops or it errors,
// it is restarted with exponential backoff until ctx is cancelled.
// The returned WaitGroup completes once both have stopped.
func Start(
	ctx context.Context,
	store SQLStore,
	brokerURL string,
	pollDelay time.Duration,
	sender Sender,
) *sync.WaitGroup {
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		supervise(ctx, "outbox dispatcher", func(ctx context.Context) error {
			return RunDispatcher(ctx, store, brokerURL, pollDelay)
		})
	}()
	go func() {
		defer wg.Done()
		supervise(ctx, "notification worker", func(ctx context.Context) error {
			return RunWorker(ctx, store, brokerURL, sender)
		})
	}()
	return &wg
}

func supervise(ctx context.Context, name string, run func(context.Context) error) {
	const (
		initialBackoff = time.Second
		maxBackoff     = 30 * time.Second
		healthyAfter   = time.Minute
	)
	backoff := initialBackoff
	for {
		started := time.Now()
		err := run(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("email queue %s stopped: %v; restarting in %s", name, err, backoff)
		} else {
			log.Printf("email queue %s exited; restarting in %s", name, backoff)
		}
		// A run that stayed up for a while counts as healthy: reset backoff.
		if time.Since(started) > healthyAfter {
			backoff = initialBackoff
		}
		if Wait(ctx, backoff) != nil {
			return
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}
