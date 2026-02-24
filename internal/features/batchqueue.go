package features

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// JobStatus represents the state of a batch job.
type JobStatus string

const (
	JobPending    JobStatus = "pending"
	JobProcessing JobStatus = "processing"
	JobCompleted  JobStatus = "completed"
	JobFailed     JobStatus = "failed"
)

// BatchJob represents a single queued LLM request.
type BatchJob struct {
	ID        string         `json:"id"`
	BatchID   string         `json:"batch_id,omitempty"`
	Priority  string         `json:"priority"`
	Status    JobStatus      `json:"status"`
	Request   *provider.Request `json:"-"`
	ReqJSON   json.RawMessage `json:"request"`
	Response  *provider.Response `json:"-"`
	RespJSON  json.RawMessage `json:"response,omitempty"`
	Error     string         `json:"error,omitempty"`
	Attempts  int            `json:"attempts"`
	CreatedAt time.Time      `json:"created_at"`
	StartedAt *time.Time     `json:"started_at,omitempty"`
	DoneAt    *time.Time     `json:"done_at,omitempty"`
}

// BatchQueueManager manages the async job queue.
type BatchQueueManager struct {
	mu          sync.RWMutex
	jobs        map[string]*BatchJob
	queue       []*BatchJob // pending jobs sorted by priority
	concurrency int
	maxRetries  int
	running     atomic.Int32
	totalJobs   atomic.Int64
	completed   atomic.Int64
	failed      atomic.Int64
	handler     proxy.Handler
}

// NewBatchQueue creates a batch queue from config.
func NewBatchQueue(cfg config.BatchQueueConfig) *BatchQueueManager {
	conc := cfg.Concurrency.Default
	if conc <= 0 {
		conc = 5
	}
	retries := cfg.Retry.MaxAttempts
	if retries <= 0 {
		retries = 3
	}

	return &BatchQueueManager{
		jobs:        make(map[string]*BatchJob),
		concurrency: conc,
		maxRetries:  retries,
	}
}

// SetHandler sets the request handler for processing jobs.
func (bq *BatchQueueManager) SetHandler(h proxy.Handler) {
	bq.handler = h
}

// Submit adds a job to the queue. Returns the job ID.
func (bq *BatchQueueManager) Submit(req *provider.Request, priority string) string {
	if priority == "" {
		priority = "normal"
	}

	id := fmt.Sprintf("job_%d_%d", time.Now().UnixNano(), bq.totalJobs.Add(1))

	job := &BatchJob{
		ID:        id,
		Priority:  priority,
		Status:    JobPending,
		Request:   req,
		CreatedAt: time.Now(),
	}

	bq.mu.Lock()
	bq.jobs[id] = job
	bq.insertByPriority(job)
	bq.mu.Unlock()

	// Try to process immediately if capacity available
	go bq.processNext()

	return id
}

// SubmitBatch adds multiple jobs and returns a batch ID.
func (bq *BatchQueueManager) SubmitBatch(requests []*provider.Request, priority string) (string, []string) {
	batchID := fmt.Sprintf("batch_%d", time.Now().UnixNano())
	var ids []string
	for _, req := range requests {
		id := bq.Submit(req, priority)
		bq.mu.Lock()
		if job, ok := bq.jobs[id]; ok {
			job.BatchID = batchID
		}
		bq.mu.Unlock()
		ids = append(ids, id)
	}
	return batchID, ids
}

// GetJob retrieves a job by ID.
func (bq *BatchQueueManager) GetJob(id string) *BatchJob {
	bq.mu.RLock()
	defer bq.mu.RUnlock()
	return bq.jobs[id]
}

// GetBatchJobs retrieves all jobs for a batch.
func (bq *BatchQueueManager) GetBatchJobs(batchID string) []*BatchJob {
	bq.mu.RLock()
	defer bq.mu.RUnlock()
	var jobs []*BatchJob
	for _, job := range bq.jobs {
		if job.BatchID == batchID {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

// Stats returns queue statistics.
func (bq *BatchQueueManager) Stats() map[string]any {
	bq.mu.RLock()
	pending := len(bq.queue)
	bq.mu.RUnlock()

	return map[string]any{
		"total_jobs": bq.totalJobs.Load(),
		"completed":  bq.completed.Load(),
		"failed":     bq.failed.Load(),
		"pending":    pending,
		"running":    bq.running.Load(),
	}
}

func (bq *BatchQueueManager) processNext() {
	if bq.handler == nil {
		return
	}

	if int(bq.running.Load()) >= bq.concurrency {
		return
	}

	bq.mu.Lock()
	if len(bq.queue) == 0 {
		bq.mu.Unlock()
		return
	}
	job := bq.queue[0]
	bq.queue = bq.queue[1:]
	job.Status = JobProcessing
	now := time.Now()
	job.StartedAt = &now
	bq.mu.Unlock()

	bq.running.Add(1)
	defer bq.running.Add(-1)

	for attempt := 0; attempt <= bq.maxRetries; attempt++ {
		job.Attempts = attempt + 1
		resp, err := bq.handler(context.Background(), job.Request)
		if err == nil {
			bq.mu.Lock()
			job.Status = JobCompleted
			job.Response = resp
			doneAt := time.Now()
			job.DoneAt = &doneAt
			bq.mu.Unlock()
			bq.completed.Add(1)
			log.Printf("batchqueue: job %s completed (attempt %d)", job.ID, attempt+1)
			go bq.processNext()
			return
		}

		log.Printf("batchqueue: job %s attempt %d failed: %v", job.ID, attempt+1, err)

		if attempt < bq.maxRetries {
			// Exponential backoff
			time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
		}
	}

	bq.mu.Lock()
	job.Status = JobFailed
	job.Error = "max retries exceeded"
	doneAt := time.Now()
	job.DoneAt = &doneAt
	bq.mu.Unlock()
	bq.failed.Add(1)

	go bq.processNext()
}

func (bq *BatchQueueManager) insertByPriority(job *BatchJob) {
	prio := priorityValue(job.Priority)
	inserted := false
	for i, existing := range bq.queue {
		if priorityValue(existing.Priority) < prio {
			bq.queue = append(bq.queue[:i+1], bq.queue[i:]...)
			bq.queue[i] = job
			inserted = true
			break
		}
	}
	if !inserted {
		bq.queue = append(bq.queue, job)
	}
}

func priorityValue(p string) int {
	switch p {
	case "urgent":
		return 3
	case "normal":
		return 2
	case "batch":
		return 1
	default:
		return 2
	}
}

// BatchQueueMiddleware returns middleware that can intercept async requests.
// Requests with X-Async: true header get queued; returns job ID immediately.
func BatchQueueMiddleware(bq *BatchQueueManager) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		bq.SetHandler(next)
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			// Check for async flag
			isAsync, _ := req.Extra["_async"].(bool)
			if !isAsync {
				if asyncStr, ok := req.Extra["X-Async"].(string); ok && asyncStr == "true" {
					isAsync = true
				}
			}

			if !isAsync {
				return next(ctx, req)
			}

			priority, _ := req.Extra["_priority"].(string)
			jobID := bq.Submit(req, priority)

			return &provider.Response{
				ID:     jobID,
				Object: "batch.job",
				Model:  req.Model,
				Choices: []provider.Choice{{
					Message: provider.Message{
						Role:    "assistant",
						Content: fmt.Sprintf(`{"job_id":"%s","status":"pending"}`, jobID),
					},
				}},
			}, nil
		}
	}
}
