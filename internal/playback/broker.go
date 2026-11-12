package playback

import "sync"

// updateBroker keeps the engine's goroutines independent of the UI. Close seals input;
// the output remains available until the caller has drained all accepted events.
type updateBroker struct {
	mu     sync.Mutex
	queue  []Update
	closed bool
	wake   chan struct{}
	out    chan Update
}

func newUpdateBroker() *updateBroker {
	b := &updateBroker{wake: make(chan struct{}, 1), out: make(chan Update, 32)}
	go b.run()
	return b
}

func (b *updateBroker) offer(u Update) {
	b.mu.Lock()
	if !b.closed {
		u.Bands = append([]float32(nil), u.Bands...)
		// A busy UI needs the latest clock/picture. Removing an earlier
		// transient and appending its replacement keeps critical event order.
		if u.Kind == UpdatePosition || u.Kind == UpdateSpectrum {
			for i := len(b.queue) - 1; i >= 0; i-- {
				if old := b.queue[i]; old.Kind == u.Kind && old.Generation == u.Generation && old.Epoch == u.Epoch {
					copy(b.queue[i:], b.queue[i+1:])
					b.queue[len(b.queue)-1] = Update{}
					b.queue = b.queue[:len(b.queue)-1]
					break
				}
			}
		}
		b.queue = append(b.queue, u)
	}
	b.mu.Unlock()
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

func (b *updateBroker) close() {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

func (b *updateBroker) run() {
	defer close(b.out)
	for {
		b.mu.Lock()
		if len(b.queue) == 0 {
			closed := b.closed
			b.mu.Unlock()
			if closed {
				return
			}
			<-b.wake
			continue
		}
		u := b.queue[0]
		b.queue[0] = Update{}
		b.queue = b.queue[1:]
		b.mu.Unlock()
		b.out <- u
	}
}
