package wvapp

import (
	"runtime"
	"sync"

	"github.com/millken/wvapp/internal/goid"
)

var (
	mainGID int64
	gidOnce sync.Once
)

type Scheduler struct {
	tasks       chan func()
	once        sync.Once
	initialized bool
}

func NewScheduler() *Scheduler {
	s := &Scheduler{
		tasks: make(chan func(), 64),
	}
	return s
}

func (s *Scheduler) RunInMainThread(f func()) {
	if !s.initialized {
		s.Start()
	}
	s.tasks <- f
}

// Run a function in the main thread and return its result
func (s *Scheduler) RunInMainThreadWithResult(f func() any) any {
	gidOnce.Do(func() {
		mainGID = goid.Goid()
	})
	if goid.Goid() == mainGID {
		return f()
	}
	resultCh := make(chan any, 1)
	s.tasks <- func() {
		resultCh <- f()
	}
	return <-resultCh
}

func (s *Scheduler) Start() {
	s.once.Do(func() {
		runtime.LockOSThread()
		s.initialized = true
	})
}

// PollTasks executes all pending tasks without blocking
func (s *Scheduler) PollTasks() {
	for {
		select {
		case task := <-s.tasks:
			if task != nil {
				task()
			}
		default:
			return
		}
	}
}
