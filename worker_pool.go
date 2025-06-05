package wvapp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
	// "time" // Uncomment for example tasks
)

// CallPayload defines the structure of messages from JavaScript.
// Ensure this matches the structure sent by your runtime.js/goCall.
type CallPayload struct {
	Func      string `json:"func"`
	Args      []any  `json:"args"`
	PromiseID int    `json:"promiseId,omitempty"` // omitempty if JS doesn't always send it
}

type HandlerFunc func(ctx context.Context, wv *Webview, args []any) (result any, err error)

// Job represents a task to be executed by a worker.
type Job struct {
	Webview *Webview    // The wvapp instance to interact with (e.g., for EvalJS)
	Payload CallPayload // The original payload from JavaScript
	Handler HandlerFunc // The Go function to execute
}

// WorkerPool manages a pool of worker goroutines.
type WorkerPool struct {
	workerCount int
	jobQueue    chan Job
	wg          sync.WaitGroup
	quit        chan struct{} // Channel to signal workers to stop
}

// NewWorkerPool creates and starts a new worker pool.
func NewWorkerPool(workerCount int, queueSize int) *WorkerPool {
	if workerCount <= 0 {
		workerCount = 4 // Sensible default
	}
	if queueSize <= 0 {
		queueSize = 100 // Sensible default
	}

	pool := &WorkerPool{
		workerCount: workerCount,
		jobQueue:    make(chan Job, queueSize), // Buffered channel
		quit:        make(chan struct{}),
	}

	pool.startWorkers()
	return pool
}

// startWorkers launches the worker goroutines.
func (wp *WorkerPool) startWorkers() {
	for i := range wp.workerCount {
		wp.wg.Add(1)
		go func(workerID int) {
			defer wp.wg.Done()
			// fmt.Printf("Worker %d started\n", workerID)
			for {
				select {
				case job, ok := <-wp.jobQueue:
					if !ok {
						// jobQueue has been closed, worker should exit.
						// fmt.Printf("Worker %d stopping as job queue closed\n", workerID)
						return
					}
					wp.processJob(job)
				case <-wp.quit:
					// fmt.Printf("Worker %d received quit signal, stopping\n", workerID)
					return
				}
			}
		}(i)
	}
}

// processJob executes a single job and sends the result/error back to JavaScript.
func (wp *WorkerPool) processJob(job Job) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Panic recovered in worker processing job", "function", job.Payload.Func, "panic", r)
			// Stack trace can be obtained using runtime.ReadTrace or similar if needed
			if job.Payload.PromiseID != 0 { // If JS expects a response
				errorMsgJSON, _ := json.Marshal(fmt.Sprintf("Panic occurred while processing function '%s': %v", job.Payload.Func, r))
				rejectScript := fmt.Sprintf("window._rejectWebviewPromise(%d, %s);", job.Payload.PromiseID, string(errorMsgJSON))
				if job.Webview != nil { // Ensure wvapp instance is valid
					job.Webview.EvalJS(rejectScript)
				}
			}
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Set a timeout for job execution
	defer cancel()                                                           // Ensure the context is cancelled after job execution
	result, err := job.Handler(ctx, job.Webview, job.Payload.Args)

	slog.Debug("Processing job", "function", job.Payload.Func, "args", job.Payload.Args, "result", result, "error", err)
	// If PromiseID is 0 or not set, JS might not be expecting a specific promise resolution.
	// Adjust this condition based on how your JS `goCall` sends PromiseID.
	// If goCall *always* sends a promiseId when expectResponse=true, then this check is fine.
	if job.Payload.PromiseID == 0 { // Assuming 0 is not a valid promise ID from JS
		if err != nil {
			fmt.Printf("Error in fire-and-forget job %s: %v\n", job.Payload.Func, err)
		}
		return // No specific promise to resolve/reject
	}

	if err != nil {
		// Marshal the error message string to ensure it's a valid JSON string for JS
		errorMsgJSON, _ := json.Marshal(err.Error())
		rejectScript := fmt.Sprintf("window._rejectWebviewPromise(%d, %s);", job.Payload.PromiseID, string(errorMsgJSON))
		job.Webview.EvalJS(rejectScript)
	} else {
		resultJSON, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			// Failed to marshal the successful result, so reject the promise
			errorMsgJSON, _ := json.Marshal(fmt.Sprintf("Error marshalling result for %s: %v", job.Payload.Func, marshalErr))
			rejectScript := fmt.Sprintf("window._rejectWebviewPromise(%d, %s);", job.Payload.PromiseID, string(errorMsgJSON))
			job.Webview.EvalJS(rejectScript)
			return
		}
		resolveScript := fmt.Sprintf("window._resolveWebviewPromise(%d, %s);", job.Payload.PromiseID, string(resultJSON))
		slog.Debug("Job completed successfully", "function", job.Payload.Func, "resultJSON", resultJSON)
		job.Webview.EvalJS(resolveScript)
	}
}

// Submit adds a job to the worker pool's queue.
// It returns an error if the pool is shutting down or the queue is full.
func (wp *WorkerPool) Submit(job Job) error {
	select {
	case <-wp.quit:
		return fmt.Errorf("worker pool is shutting down, cannot submit job for %s", job.Payload.Func)
	default:
		// Non-blocking attempt to send to jobQueue
		select {
		case wp.jobQueue <- job:
			return nil
		case <-wp.quit: // Check quit again in case it was triggered during the outer select
			return fmt.Errorf("worker pool is shutting down, cannot submit job for %s", job.Payload.Func)
		default:
			// Queue is full
			return fmt.Errorf("worker pool job queue is full (capacity: %d), cannot submit job for %s", cap(wp.jobQueue), job.Payload.Func)
		}
	}
}

// Shutdown gracefully stops all workers.
// It first signals workers to stop accepting new jobs, then closes the job queue,
// and finally waits for all active jobs to complete.
func (wp *WorkerPool) Shutdown() {
	// fmt.Println("Worker pool: Initiating shutdown...")
	close(wp.quit)     // Signal workers to stop their loops after current job (if any from select)
	close(wp.jobQueue) // Close job queue; workers will exit when queue is empty
	wp.wg.Wait()       // Wait for all worker goroutines to finish
	// fmt.Println("Worker pool: Shutdown complete.")
}

// Global instance of the worker pool.
var globalWorkerPool *WorkerPool
var globalWorkerPoolOnce sync.Once

// UserFunctionRegistry stores the Go functions that can be called from JavaScript.
// The key is the function name (string) as called from JavaScript.
// The value is the Go function that handles the call.
var UserFunctionRegistry = make(map[string]HandlerFunc)

// InitializeGlobalWorkerPool creates the global worker pool.
// This should be called once during application startup.
func InitializeGlobalWorkerPool(workerCount int, queueSize int) {
	globalWorkerPoolOnce.Do(func() {
		globalWorkerPool = NewWorkerPool(workerCount, queueSize)
	})
}

// ShutdownGlobalWorkerPool stops the global worker pool.
// This should be called during application shutdown to ensure graceful termination.
func ShutdownGlobalWorkerPool() {
	if globalWorkerPool != nil {
		globalWorkerPool.Shutdown()
	}
}

/*
// Example of registering a function (e.g., in an init() block or setup function)
func init() {
    UserFunctionRegistry["getSystemTime"] = func(wv *Webview, args []any) (result any, err error) {
        // time.Sleep(2 * time.Second) // Simulate a delay
        return time.Now().Format(time.RFC3339), nil
    }

    UserFunctionRegistry["echoArgs"] = func(wv *Webview, args []any) (result any, err error) {
        if len(args) == 0 {
            return nil, fmt.Errorf("echoArgs expects at least one argument")
        }
        return args, nil // Echo back all arguments
    }

    UserFunctionRegistry["taskWithError"] = func(wv *Webview, args []any) (result any, err error) {
        // time.Sleep(1 * time.Second)
        return nil, fmt.Errorf("simulated error from taskWithError")
    }
}
*/
