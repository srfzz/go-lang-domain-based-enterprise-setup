package utils

import (
	"sync"
)

// Task represents a unit of work for the worker pool.
type Task func()

// WorkerPool processes tasks concurrently with a fixed number of workers.
// Prevents goroutine explosion from per-request background jobs (e.g., audit logs).
type WorkerPool struct {
	tasks  chan Task
	wg     sync.WaitGroup
	closed chan struct{}
}

func NewWorkerPool(numWorkers int, bufferSize int) *WorkerPool {
	wp := &WorkerPool{
		tasks:  make(chan Task, bufferSize),
		closed: make(chan struct{}),
	}
	for i := 0; i < numWorkers; i++ {
		wp.wg.Add(1)
		go func() {
			defer wp.wg.Done()
			for {
				select {
				case task, ok := <-wp.tasks:
					if !ok {
						return
					}
					task()
				case <-wp.closed:
					return
				}
			}
		}()
	}
	return wp
}

func (wp *WorkerPool) Submit(task Task) {
	select {
	case wp.tasks <- task:
	default:
		// Queue full; drop the task instead of blocking
	}
}

func (wp *WorkerPool) Shutdown() {
	close(wp.closed)
	wp.wg.Wait()
}
