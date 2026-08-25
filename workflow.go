package temporalsentryinterceptor

import (
	"context"

	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/workflow"
)

// TemporalWorkflowInterceptor provides Sentry error and panic reporting for Temporal workflow executions.
type TemporalWorkflowInterceptor struct {
	sentryActivities    sentryActivities
	filterWorkflowError FilterWorkflowErrorFunc
	filterWorkflowPanic FilterWorkflowPanicFunc
	interceptor.WorkflowInboundInterceptorBase
}

// InterceptWorkflow creates and returns a workflow interceptor with Sentry integration.
func (s TemporalWorkerInterceptor) InterceptWorkflow(
	_ workflow.Context, next interceptor.WorkflowInboundInterceptor,
) interceptor.WorkflowInboundInterceptor {
	return &TemporalWorkflowInterceptor{
		sentryActivities:    s.sentryActivities,
		filterWorkflowError: s.options.filterWorkflowError,
		filterWorkflowPanic: s.options.filterWorkflowPanic,
		WorkflowInboundInterceptorBase: interceptor.WorkflowInboundInterceptorBase{
			Next: next,
		},
	}
}

// ExecuteWorkflow intercepts workflow execution and reports errors/panics to Sentry.
func (s *TemporalWorkflowInterceptor) ExecuteWorkflow(
	ctx workflow.Context, in *interceptor.ExecuteWorkflowInput,
) (any, error) {
	info := workflow.GetInfo(ctx)
	defer s.captureWorkflowPanicToSentry(ctx, "WorkflowExecuteWorkflow", in.Args, info)()

	result, err := s.Next.ExecuteWorkflow(ctx, in)
	if err != nil {
		s.captureWorkflowErrorToSentry(ctx, err, "WorkflowExecuteWorkflow", in.Args, info)
	}

	return result, err
}

// HandleSignal intercepts signal handling and reports errors/panics to Sentry.
func (s *TemporalWorkflowInterceptor) HandleSignal(ctx workflow.Context, in *interceptor.HandleSignalInput) error {
	info := workflow.GetInfo(ctx)
	defer s.captureWorkflowPanicToSentry(ctx, "WorkflowHandleSignalPanic", []any{in.Arg}, info)()

	err := s.Next.HandleSignal(ctx, in)
	if err != nil {
		s.captureWorkflowErrorToSentry(ctx, err, "WorkflowHandleSignal", []any{in.Arg}, info)
	}

	return err
}

// HandleQuery intercepts query handling and reports errors/panics to Sentry.
func (s *TemporalWorkflowInterceptor) HandleQuery(ctx workflow.Context, in *interceptor.HandleQueryInput) (any, error) {
	info := workflow.GetInfo(ctx)
	defer s.captureWorkflowPanicToSentry(ctx, "WorkflowHandleQueryPanic", in.Args, info)()

	result, err := s.Next.HandleQuery(ctx, in)
	if err != nil {
		s.captureWorkflowErrorToSentry(ctx, err, "WorkflowHandleQuery", in.Args, info)
	}

	return result, err
}

// ValidateUpdate intercepts update validation and reports errors/panics to Sentry.
func (s *TemporalWorkflowInterceptor) ValidateUpdate(ctx workflow.Context, in *interceptor.UpdateInput) error {
	info := workflow.GetInfo(ctx)
	defer s.captureWorkflowPanicToSentry(ctx, "WorkflowValidateUpdatePanic", in.Args, info)()

	err := s.Next.ValidateUpdate(ctx, in)
	if err != nil {
		s.captureWorkflowErrorToSentry(ctx, err, "WorkflowValidateUpdate", in.Args, info)
	}

	return err
}

// ExecuteUpdate intercepts update execution and reports errors/panics to Sentry.
func (s *TemporalWorkflowInterceptor) ExecuteUpdate(ctx workflow.Context, in *interceptor.UpdateInput) (any, error) {
	info := workflow.GetInfo(ctx)
	defer s.captureWorkflowPanicToSentry(ctx, "WorkflowExecuteUpdatePanic", in.Args, info)()

	result, err := s.Next.ExecuteUpdate(ctx, in)
	if err != nil {
		s.captureWorkflowErrorToSentry(ctx, err, "WorkflowExecuteUpdate", in.Args, info)
	}

	return result, err
}

// captureWorkflowErrorToSentry reports workflow errors to Sentry inline, from the
// workflow goroutine.
//
// Reporting must not produce workflow commands: an earlier version ran a local
// activity here, whose MarkerRecorded history event — guarded by IsReplaying —
// was emitted on the original execution and never re-issued on replay,
// permanently failing the run with a nondeterminism error (TMPRL1100).
// CaptureException only enqueues the event on Sentry's async transport, so it
// never blocks the workflow task. IsReplaying now guards only against reporting
// the same error again during replays, the sole side effect it may guard.
func (s *TemporalWorkflowInterceptor) captureWorkflowErrorToSentry(
	ctx workflow.Context, err error, eventName string, req []any, info *workflow.Info,
) {
	if workflow.IsReplaying(ctx) {
		return
	}

	if s.filterWorkflowError != nil && s.filterWorkflowError(err, req, info) {
		return
	}

	input := ReportErrorInput{Error: err, EventName: eventName, Request: req, WorkflowInfo: info}
	_ = s.sentryActivities.ReportError(context.Background(), input)
}

// captureWorkflowPanicToSentry returns a deferred function that reports workflow
// panics to Sentry inline (see captureWorkflowErrorToSentry for why no local
// activity is involved) and always re-raises the panic, so that neither
// filtering nor replaying can swallow it.
func (s *TemporalWorkflowInterceptor) captureWorkflowPanicToSentry(
	ctx workflow.Context, eventName string, req []any, info *workflow.Info,
) func() {
	return func() {
		r := recover()
		if r == nil {
			return
		}

		filtered := s.filterWorkflowPanic != nil && s.filterWorkflowPanic(r, req, info)
		if !workflow.IsReplaying(ctx) && !filtered {
			input := ReportPanicInput{Panic: r, EventName: eventName, Request: req, WorkflowInfo: info}
			_ = s.sentryActivities.ReportPanic(context.Background(), input)
		}

		panic(r)
	}
}
