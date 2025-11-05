package wvapp

import (
	"sync"
	"sync/atomic"

	"github.com/millken/goid"
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
	// 使用 once.Do 做惰性初始化，避免竞态读取 initialized
	s.Start()
	s.tasks <- f
}

// Run a function in the main thread and return its result
func (s *Scheduler) RunInMainThreadWithResult(f func() any) any {
	// 若主线程尚未登记（Run() 未启动），直接在当前 goroutine 执行，避免阻塞
	if atomic.LoadInt64(&mainGID) == 0 || goid.Goid() == atomic.LoadInt64(&mainGID) {
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
