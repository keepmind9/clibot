package bot

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewProcessPool_NilWhenZero(t *testing.T) {
	assert.Nil(t, NewProcessPool(0))
	assert.Nil(t, NewProcessPool(-1))
}

func TestProcessPool_NilAcquireRelease(t *testing.T) {
	var pool *ProcessPool
	assert.NotPanics(t, func() { pool.Acquire() })
	assert.NotPanics(t, func() { pool.Release() })
	assert.Equal(t, 0, pool.Cap())
}

func TestProcessPool_Cap(t *testing.T) {
	p := NewProcessPool(5)
	assert.Equal(t, 5, p.Cap())
}

func TestProcessPool_BasicAcquireRelease(t *testing.T) {
	p := NewProcessPool(2)
	assert.NotPanics(t, func() {
		p.Acquire()
		p.Acquire()
		p.Release()
		p.Release()
	})
}

func TestProcessPool_BlocksAtCapacity(t *testing.T) {
	p := NewProcessPool(1)
	p.Acquire()

	var unblocked atomic.Bool
	done := make(chan struct{})
	go func() {
		p.Acquire()
		unblocked.Store(true)
		close(done)
	}()

	assert.False(t, unblocked.Load(), "should block when pool is full")
	p.Release()
	<-done
	assert.True(t, unblocked.Load(), "should unblock after release")
}

func TestProcessPool_ConcurrentAcquireRelease(t *testing.T) {
	p := NewProcessPool(3)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.Acquire()
			p.Release()
		}()
	}
	wg.Wait()
	// All goroutines completed without deadlock
}
