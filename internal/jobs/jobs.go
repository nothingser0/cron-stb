package jobs

import (
	"context"
	"fmt"
	"sync"
)

type contextKey string

const (
	ManualRunKey contextKey = "manual_run"
	DryRunKey    contextKey = "dry_run"
)

type Job interface {
	Name() string
	Description() string
	Schedule() string
	Run(ctx context.Context) error
}

var (
	registryLock sync.RWMutex
	jobRegistry  = make(map[string]Job)
	jobList      []Job
)

func Register(job Job) {
	registryLock.Lock()
	defer registryLock.Unlock()

	name := job.Name()
	if _, exists := jobRegistry[name]; exists {
		panic(fmt.Sprintf("job %s is already registered", name))
	}
	jobRegistry[name] = job
	jobList = append(jobList, job)
}

func GetAllJobs() []Job {
	registryLock.RLock()
	defer registryLock.RUnlock()
	return append([]Job(nil), jobList...)
}

func GetJob(name string) (Job, bool) {
	registryLock.RLock()
	defer registryLock.RUnlock()
	j, ok := jobRegistry[name]
	return j, ok
}
