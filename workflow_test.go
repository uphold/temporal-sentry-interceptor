package temporalsentryinterceptor

import (
	"context"
	"errors"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func failingWorkflow(_ workflow.Context) error {
	return errors.New("routine workflow failure")
}

func failingQueryWorkflow(ctx workflow.Context) error {
	if err := workflow.SetQueryHandler(ctx, "failingQuery", func() (string, error) {
		return "", errors.New("query handler failure")
	}); err != nil {
		return err
	}

	return nil
}

func panickingWorkflow(_ workflow.Context) error {
	panic("workflow panic")
}

// newReportingTestEnv returns a workflow test environment with the Sentry
// interceptor installed, plus flags recording whether reporting reached Sentry
// scope configuration and whether any local activity ran.
func newReportingTestEnv(t *testing.T) (env *testsuite.TestWorkflowEnvironment, reported, localActivityRan *bool) {
	t.Helper()

	reported = new(bool)
	localActivityRan = new(bool)

	sentryInterceptor := New(WithConfigureSentryScope(func(
		_ context.Context, _ []any, _ string, _ *activity.Info, _ *workflow.Info,
	) func(scope *sentry.Scope) {
		*reported = true

		return func(_ *sentry.Scope) {}
	}))

	var ts testsuite.WorkflowTestSuite
	env = ts.NewTestWorkflowEnvironment()
	env.SetWorkerOptions(worker.Options{
		Interceptors: []interceptor.WorkerInterceptor{sentryInterceptor},
	})
	env.SetOnLocalActivityStartedListener(func(_ *activity.Info, _ context.Context, _ []any) {
		*localActivityRan = true
	})

	return env, reported, localActivityRan
}

// Workflow error and panic reporting must not execute a local activity: its
// MarkerRecorded history event, emitted only when not replaying, permanently
// breaks replay determinism (TMPRL1100) for any run that later replays.
func TestWorkflowErrorReportingEmitsNoCommands(t *testing.T) {
	env, reported, localActivityRan := newReportingTestEnv(t)
	env.RegisterWorkflow(failingWorkflow)

	env.ExecuteWorkflow(failingWorkflow)

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	assert.True(t, *reported, "workflow error should be reported to Sentry")
	assert.False(t, *localActivityRan, "reporting must not run a local activity — its marker breaks replay determinism")
}

func TestWorkflowPanicReportingEmitsNoCommandsAndRepanics(t *testing.T) {
	env, reported, localActivityRan := newReportingTestEnv(t)
	env.RegisterWorkflow(panickingWorkflow)

	env.ExecuteWorkflow(panickingWorkflow)

	require.True(t, env.IsWorkflowCompleted())
	workflowErr := env.GetWorkflowError()
	require.Error(t, workflowErr, "the panic must propagate, not be swallowed by the interceptor")
	assert.Contains(t, workflowErr.Error(), "workflow panic")
	assert.True(t, *reported, "workflow panic should be reported to Sentry")
	assert.False(t, *localActivityRan, "reporting must not run a local activity — its marker breaks replay determinism")
}

func TestFilteredWorkflowPanicStillPropagates(t *testing.T) {
	sentryInterceptor := New(WithFilterWorkflowPanic(func(_ any, _ []any, _ *workflow.Info) bool {
		return true
	}))

	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()
	env.SetWorkerOptions(worker.Options{
		Interceptors: []interceptor.WorkerInterceptor{sentryInterceptor},
	})
	env.RegisterWorkflow(panickingWorkflow)

	env.ExecuteWorkflow(panickingWorkflow)

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError(), "a filtered panic is not reported but must still fail the workflow")
}

// Query-handler errors are captured without a replay guard: queries are never
// re-delivered by replay, and a cold worker serving one after rebuilding state
// from history still has IsReplaying()==true — a blanket guard would silently
// drop the error. The test environment cannot simulate that cold-worker state,
// so this asserts the capture path itself; the replay-flag reasoning is
// documented on captureWorkflowErrorToSentry.
func TestQueryErrorIsReported(t *testing.T) {
	env, reported, localActivityRan := newReportingTestEnv(t)
	env.RegisterWorkflow(failingQueryWorkflow)

	env.ExecuteWorkflow(failingQueryWorkflow)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	_, err := env.QueryWorkflow("failingQuery")
	require.Error(t, err)
	assert.True(t, *reported, "query handler error should be reported to Sentry")
	assert.False(t, *localActivityRan)
}
