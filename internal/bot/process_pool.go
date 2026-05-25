package bot

// ProcessPool limits concurrent CLI runs using a buffered channel as a counting
// semaphore. A nil or zero-cap pool is a no-op (unlimited concurrency).
type ProcessPool struct {
	sem chan struct{}
	cap int
}

// NewProcessPool creates a pool with the given capacity.
// Returns nil if maxRuns <= 0 (no limit).
func NewProcessPool(maxRuns int) *ProcessPool {
	if maxRuns <= 0 {
		return nil
	}
	return &ProcessPool{
		sem: make(chan struct{}, maxRuns),
		cap: maxRuns,
	}
}

// Acquire blocks until a permit is available.
// No-op if the pool is nil.
func (p *ProcessPool) Acquire() {
	if p == nil {
		return
	}
	p.sem <- struct{}{}
}

// Release returns a permit to the pool.
// No-op if the pool is nil.
func (p *ProcessPool) Release() {
	if p == nil {
		return
	}
	<-p.sem
}

// Cap returns the pool capacity, or 0 if unlimited (nil pool).
func (p *ProcessPool) Cap() int {
	if p == nil {
		return 0
	}
	return p.cap
}
