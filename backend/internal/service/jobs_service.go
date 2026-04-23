package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

const (
	JobCapabilityTextBasic       = "text.basic"
	JobCapabilityImageGeneration = "image.generation"

	JobStatusQueued    = "queued"
	JobStatusRunning   = "running"
	JobStatusSucceeded = "succeeded"
	JobStatusFailed    = "failed"
	JobStatusCancelled = "cancelled"

	JobExecutorLocal    = "bf2025-local"
	JobExecutorPyWorker = "py-worker"

	defaultJobRemoteTimeout = 15 * time.Second
)

type CreateJobInput struct {
	Capability     string            `json:"capability"`
	Input          map[string]any    `json:"input,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	PreferExecutor string            `json:"prefer_executor,omitempty"`
}

type Job struct {
	JobID                string            `json:"job_id"`
	RequestedCapability  string            `json:"requested_capability"`
	SelectedExecutor     string            `json:"selected_executor,omitempty"`
	SelectedExecutorKind string            `json:"selected_executor_kind,omitempty"`
	Status               string            `json:"status"`
	Error                string            `json:"error,omitempty"`
	DispatchTrace        []string          `json:"dispatch_trace,omitempty"`
	Metadata             map[string]string `json:"metadata,omitempty"`
	Result               any               `json:"result,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`
	UpdatedAt            time.Time         `json:"updated_at"`
}

type JobsService struct {
	mu            sync.RWMutex
	jobs          map[string]Job
	localExecutor jobsExecutor
	remoteWorkers []jobsExecutor
}

type jobsExecutor interface {
	Name() string
	Kind() string
	Supports(capability string) bool
	Execute(ctx context.Context, job Job, input CreateJobInput) (any, error)
}

type localJobsExecutor struct {
	name         string
	capabilities map[string]struct{}
}

type remoteJobsExecutor struct {
	name         string
	baseURL      string
	timeout      time.Duration
	client       *http.Client
	capabilities map[string]struct{}
}

type remoteJobExecuteRequest struct {
	JobID       string            `json:"job_id"`
	Capability  string            `json:"capability"`
	Input       map[string]any    `json:"input,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	RequestedBy string            `json:"requested_by,omitempty"`
}

type remoteJobExecuteResponse struct {
	Status string `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
	Result any    `json:"result,omitempty"`
}

type responseEnvelope struct {
	Code    int             `json:"code"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
}

func NewJobsService() *JobsService {
	return newJobsService(defaultJobRemoteTimeout, []jobsExecutor{
		newRemoteJobsExecutor(JobExecutorPyWorker, strings.TrimSpace(getenv("SUB2API_PY_WORKER_URL")), defaultJobRemoteTimeout, []string{
			JobCapabilityTextBasic,
			JobCapabilityImageGeneration,
		}),
	})
}

func newJobsService(remoteTimeout time.Duration, remoteWorkers []jobsExecutor) *JobsService {
	if remoteTimeout <= 0 {
		remoteTimeout = defaultJobRemoteTimeout
	}
	filteredWorkers := make([]jobsExecutor, 0, len(remoteWorkers))
	for _, worker := range remoteWorkers {
		if worker == nil {
			continue
		}
		filteredWorkers = append(filteredWorkers, worker)
	}
	return &JobsService{
		jobs: make(map[string]Job),
		localExecutor: newLocalJobsExecutor(JobExecutorLocal, []string{
			JobCapabilityTextBasic,
			JobCapabilityImageGeneration,
		}),
		remoteWorkers: filteredWorkers,
	}
}

func (s *JobsService) CreateJob(ctx context.Context, input CreateJobInput) (Job, error) {
	capability := strings.TrimSpace(input.Capability)
	if capability == "" {
		return Job{}, infraerrors.BadRequest("JOB_CAPABILITY_REQUIRED", "capability is required")
	}
	input.Capability = capability

	now := time.Now().UTC()
	job := Job{
		JobID:               uuid.NewString(),
		RequestedCapability: capability,
		Status:              JobStatusQueued,
		Metadata:            cloneStringMap(input.Metadata),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	s.storeJob(job)

	executionPlan := s.buildExecutionPlan(capability, strings.TrimSpace(input.PreferExecutor))
	if len(executionPlan) == 0 {
		job.Status = JobStatusFailed
		job.Error = "no executor available for requested capability"
		job.UpdatedAt = time.Now().UTC()
		s.storeJob(job)
		log.Printf("[jobs] dispatch mode=hard_fail job_id=%s capability=%s reason=no_executor", job.JobID, capability)
		return job, nil
	}

	var lastErr error
	for idx, executor := range executionPlan {
		job.Status = JobStatusRunning
		job.SelectedExecutor = executor.Name()
		job.SelectedExecutorKind = executor.Kind()
		job.DispatchTrace = append(job.DispatchTrace, executor.Name())
		job.UpdatedAt = time.Now().UTC()
		s.storeJob(job)

		mode := executor.Kind()
		log.Printf("[jobs] dispatch mode=%s job_id=%s capability=%s executor=%s", mode, job.JobID, capability, executor.Name())

		result, execErr := executor.Execute(ctx, job, input)
		if execErr == nil {
			job.Status = JobStatusSucceeded
			job.Error = ""
			job.Result = result
			job.UpdatedAt = time.Now().UTC()
			s.storeJob(job)
			return job, nil
		}

		lastErr = execErr
		if idx+1 < len(executionPlan) {
			nextExecutor := executionPlan[idx+1]
			log.Printf("[jobs] dispatch mode=fallback job_id=%s capability=%s from=%s to=%s cause=%v", job.JobID, capability, executor.Name(), nextExecutor.Name(), execErr)
			continue
		}
	}

	job.Status = JobStatusFailed
	job.Error = lastErr.Error()
	job.UpdatedAt = time.Now().UTC()
	s.storeJob(job)
	log.Printf("[jobs] dispatch mode=hard_fail job_id=%s capability=%s executor=%s error=%v", job.JobID, capability, job.SelectedExecutor, lastErr)
	return job, nil
}

func (s *JobsService) GetJob(_ context.Context, jobID string) (Job, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return Job{}, infraerrors.BadRequest("JOB_ID_REQUIRED", "job_id is required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[jobID]
	if !ok {
		return Job{}, infraerrors.NotFound("JOB_NOT_FOUND", "job not found")
	}
	return cloneJob(job), nil
}

func (s *JobsService) buildExecutionPlan(capability, preferExecutor string) []jobsExecutor {
	var remoteCandidate jobsExecutor
	for _, worker := range s.remoteWorkers {
		if worker == nil || !worker.Supports(capability) {
			continue
		}
		if preferExecutor != "" && worker.Name() != preferExecutor {
			continue
		}
		remoteCandidate = worker
		break
	}
	if preferExecutor == JobExecutorLocal {
		if s.localExecutor != nil && s.localExecutor.Supports(capability) {
			return []jobsExecutor{s.localExecutor}
		}
		return nil
	}
	if remoteCandidate != nil {
		plan := []jobsExecutor{remoteCandidate}
		if preferExecutor == "" && s.localExecutor != nil && s.localExecutor.Supports(capability) {
			plan = append(plan, s.localExecutor)
		}
		return plan
	}
	if s.localExecutor != nil && s.localExecutor.Supports(capability) {
		return []jobsExecutor{s.localExecutor}
	}
	return nil
}

func (s *JobsService) storeJob(job Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.JobID] = cloneJob(job)
}

func newLocalJobsExecutor(name string, capabilities []string) jobsExecutor {
	return &localJobsExecutor{
		name:         name,
		capabilities: buildCapabilitySet(capabilities),
	}
}

func newRemoteJobsExecutor(name, baseURL string, timeout time.Duration, capabilities []string) jobsExecutor {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil
	}
	if timeout <= 0 {
		timeout = defaultJobRemoteTimeout
	}
	return &remoteJobsExecutor{
		name:         name,
		baseURL:      baseURL,
		timeout:      timeout,
		client:       &http.Client{Timeout: timeout},
		capabilities: buildCapabilitySet(capabilities),
	}
}

func (e *localJobsExecutor) Name() string { return e.name }

func (e *localJobsExecutor) Kind() string { return "local" }

func (e *localJobsExecutor) Supports(capability string) bool {
	_, ok := e.capabilities[capability]
	return ok
}

func (e *localJobsExecutor) Execute(_ context.Context, job Job, input CreateJobInput) (any, error) {
	return map[string]any{
		"handled_by":  e.name,
		"mode":        "pilot_local_executor",
		"capability":  input.Capability,
		"job_id":      job.JobID,
		"input":       cloneAnyMap(input.Input),
		"metadata":    cloneStringMap(input.Metadata),
		"description": "local pilot executor placeholder completed on center",
	}, nil
}

func (e *remoteJobsExecutor) Name() string { return e.name }

func (e *remoteJobsExecutor) Kind() string { return "remote" }

func (e *remoteJobsExecutor) Supports(capability string) bool {
	_, ok := e.capabilities[capability]
	return ok
}

func (e *remoteJobsExecutor) Execute(ctx context.Context, job Job, input CreateJobInput) (any, error) {
	payload := remoteJobExecuteRequest{
		JobID:       job.JobID,
		Capability:  input.Capability,
		Input:       cloneAnyMap(input.Input),
		Metadata:    cloneStringMap(input.Metadata),
		RequestedBy: "center",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, infraerrors.InternalServer("JOB_REMOTE_MARSHAL_FAILED", "failed to encode remote job payload").WithCause(err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, e.baseURL+"/internal/jobs/execute", bytes.NewReader(body))
	if err != nil {
		return nil, infraerrors.InternalServer("JOB_REMOTE_REQUEST_BUILD_FAILED", "failed to build remote job request").WithCause(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Sub2API-Job-ID", job.JobID)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, infraerrors.ServiceUnavailable("JOB_REMOTE_REQUEST_FAILED", "remote worker request failed").WithCause(err)
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if readErr != nil {
		return nil, infraerrors.ServiceUnavailable("JOB_REMOTE_RESPONSE_READ_FAILED", "failed to read remote worker response").WithCause(readErr)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			msg = resp.Status
		}
		return nil, infraerrors.ServiceUnavailable("JOB_REMOTE_BAD_STATUS", fmt.Sprintf("remote worker returned %d: %s", resp.StatusCode, msg))
	}

	parsed, err := parseRemoteJobResponse(respBody)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(parsed.Status, JobStatusFailed) {
		if parsed.Error == "" {
			parsed.Error = "remote worker returned failed status"
		}
		return nil, infraerrors.ServiceUnavailable("JOB_REMOTE_EXECUTION_FAILED", parsed.Error)
	}
	return parsed.Result, nil
}

func parseRemoteJobResponse(body []byte) (remoteJobExecuteResponse, error) {
	var envelope responseEnvelope
	if err := json.Unmarshal(body, &envelope); err == nil && len(envelope.Data) > 0 {
		var parsed remoteJobExecuteResponse
		if err := json.Unmarshal(envelope.Data, &parsed); err == nil {
			if parsed.Status == "" {
				parsed.Status = JobStatusSucceeded
			}
			return parsed, nil
		}
	}

	var parsed remoteJobExecuteResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return remoteJobExecuteResponse{}, infraerrors.ServiceUnavailable("JOB_REMOTE_RESPONSE_INVALID", "remote worker response is not valid JSON").WithCause(err)
	}
	if parsed.Status == "" {
		parsed.Status = JobStatusSucceeded
	}
	return parsed, nil
}

func buildCapabilitySet(capabilities []string) map[string]struct{} {
	set := make(map[string]struct{}, len(capabilities))
	for _, capability := range capabilities {
		capability = strings.TrimSpace(capability)
		if capability == "" {
			continue
		}
		set[capability] = struct{}{}
	}
	return set
}

func cloneAnyMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneJob(job Job) Job {
	job.Metadata = cloneStringMap(job.Metadata)
	job.DispatchTrace = append([]string(nil), job.DispatchTrace...)
	if typed, ok := job.Result.(map[string]any); ok {
		job.Result = cloneAnyMap(typed)
	}
	return job
}
