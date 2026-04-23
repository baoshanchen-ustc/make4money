package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestJobsServiceCreateJobUsesLocalExecutorByDefault(t *testing.T) {
	svc := newJobsService(time.Second, nil)

	job, err := svc.CreateJob(context.Background(), CreateJobInput{
		Capability: JobCapabilityTextBasic,
		Input: map[string]any{
			"prompt": "hello",
		},
	})
	require.NoError(t, err)
	require.Equal(t, JobStatusSucceeded, job.Status)
	require.Equal(t, JobExecutorLocal, job.SelectedExecutor)
	require.Equal(t, "local", job.SelectedExecutorKind)
	require.Equal(t, []string{JobExecutorLocal}, job.DispatchTrace)
}

func TestJobsServiceCreateJobFallsBackToLocalWhenRemoteFails(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "worker unavailable", http.StatusServiceUnavailable)
	}))
	defer remote.Close()

	svc := newJobsService(time.Second, []jobsExecutor{
		newRemoteJobsExecutor(JobExecutorPyWorker, remote.URL, time.Second, []string{JobCapabilityImageGeneration}),
	})

	job, err := svc.CreateJob(context.Background(), CreateJobInput{
		Capability: JobCapabilityImageGeneration,
		Input: map[string]any{
			"prompt": "draw a cat",
		},
	})
	require.NoError(t, err)
	require.Equal(t, JobStatusSucceeded, job.Status)
	require.Equal(t, JobExecutorLocal, job.SelectedExecutor)
	require.Equal(t, []string{JobExecutorPyWorker, JobExecutorLocal}, job.DispatchTrace)
}

func TestJobsServiceCreateJobUsesRemoteWorkerWhenAvailable(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/internal/jobs/execute", r.URL.Path)
		var req remoteJobExecuteRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, JobCapabilityTextBasic, req.Capability)
		_ = json.NewEncoder(w).Encode(remoteJobExecuteResponse{
			Status: JobStatusSucceeded,
			Result: map[string]any{
				"handled_by": JobExecutorPyWorker,
			},
		})
	}))
	defer remote.Close()

	svc := newJobsService(time.Second, []jobsExecutor{
		newRemoteJobsExecutor(JobExecutorPyWorker, remote.URL, time.Second, []string{JobCapabilityTextBasic}),
	})

	job, err := svc.CreateJob(context.Background(), CreateJobInput{
		Capability: JobCapabilityTextBasic,
		Input: map[string]any{
			"prompt": "hello",
		},
	})
	require.NoError(t, err)
	require.Equal(t, JobStatusSucceeded, job.Status)
	require.Equal(t, JobExecutorPyWorker, job.SelectedExecutor)
	require.Equal(t, "remote", job.SelectedExecutorKind)
	require.Equal(t, []string{JobExecutorPyWorker}, job.DispatchTrace)
}

func TestJobsServiceGetJobReturnsNotFound(t *testing.T) {
	svc := newJobsService(time.Second, nil)
	_, err := svc.GetJob(context.Background(), "missing")
	require.Error(t, err)
}
