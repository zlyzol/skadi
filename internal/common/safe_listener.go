package common

import (
	"sync"
)
type ThreadSafeSliceWorker struct {
	ListenerC chan interface{}
}
type ThreadSafeSlice struct {
	sync.Mutex
    workers	[]*ThreadSafeSliceWorker
}
func (slice *ThreadSafeSlice) Push(w *ThreadSafeSliceWorker) {
    slice.Lock()
	defer slice.Unlock()
	if slice.workers == nil {
		slice.workers = []*ThreadSafeSliceWorker{}
	}
	slice.workers = append(slice.workers, w)
}
func (slice *ThreadSafeSlice) Remove(w *ThreadSafeSliceWorker) {
    slice.Lock()
	defer slice.Unlock()
	for i := 0; i < len(slice.workers); i++ {
		if slice.workers[i] == w {
			close(w.ListenerC)
			slice.workers[i] = slice.workers[len(slice.workers)-1]
			slice.workers[len(slice.workers)-1] = nil
			slice.workers = slice.workers[:len(slice.workers)-1]
		}
	}
}
func (slice *ThreadSafeSlice) Iter(routine func(*ThreadSafeSliceWorker)) {
    slice.Lock()
    defer slice.Unlock()
    for _, worker := range slice.workers {
        routine(worker)
    }
}
