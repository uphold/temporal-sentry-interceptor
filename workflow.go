package temporalsentryinterceptor

import (
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/workflow"
)

// TemporalWorkflowInterceptor provides Sentry error and panic reporting for Temporal workflow executions.
type TemporalWorkflowInterceptor struct {
	sentryActivities             sentryActivities
	filterWorkflowError          FilterWorkflowErrorFunc
	filterWorkflowPanic          FilterWorkflowPanicFunc
	workflowPanicActivityOptions workflow.LocalActivityOptions
	workflowErrorActivityOptions workflow.LocalActivityOptions
	interceptor.WorkflowInboundInterceptorBase
}

// InterceptWorkflow creates and returns a workflow interceptor with Sentry integration.
func (s TemporalWorkerInterceptor) InterceptWorkflow(
	_ workflow.Context, next interceptor.WorkflowInboundInterceptor,
) interceptor.WorkflowInboundInterceptor {
	return &TemporalWorkflowInterceptor{
		sentryActivities:             s.sentryActivities,
		filterWorkflowError:          s.options.filterWorkflowError,
		filterWorkflowPanic:          s.options.filterWorkflowPanic,
		workflowPanicActivityOptions: s.options.workflowPanicActivityOptions,
		workflowErrorActivityOptions: s.options.workflowErrorActivityOptions,
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

// captureWorkflowErrorToSentry reports workflow errors to Sentry with context and filtering.
func (s *TemporalWorkflowInterceptor) captureWorkflowErrorToSentry(
	ctx workflow.Context, err error, eventName string, req []any, info *workflow.Info,
) {
	if workflow.IsReplaying(ctx) {
		return
	}

	if s.filterWorkflowError != nil && s.filterWorkflowError(err, req, info) {
		return
	}

	disconnectedCtx, _ := workflow.NewDisconnectedContext(ctx)
	disconnectedCtx = workflow.WithLocalActivityOptions(disconnectedCtx, s.workflowErrorActivityOptions)

	input := ReportErrorInput{Error: err, EventName: eventName, Request: req, WorkflowInfo: info}
	_ = workflow.ExecuteLocalActivity(disconnectedCtx, s.sentryActivities.ReportError, input).Get(disconnectedCtx, nil)
}

// captureWorkflowPanicToSentry returns a deferred function that reports workflow panics to Sentry.
func (s *TemporalWorkflowInterceptor) captureWorkflowPanicToSentry(
	ctx workflow.Context, eventName string, req []any, info *workflow.Info,
) func() {
	return func() {
		if workflow.IsReplaying(ctx) {
			return
		}

		if r := recover(); r != nil {
			if s.filterWorkflowPanic != nil && s.filterWorkflowPanic(r, req, info) {
				return
			}

			disconnectedCtx, _ := workflow.NewDisconnectedContext(ctx)
			disconnectedCtx = workflow.WithLocalActivityOptions(disconnectedCtx, s.workflowPanicActivityOptions)
			input := ReportPanicInput{Panic: r, EventName: eventName, Request: req, WorkflowInfo: info}
			_ = workflow.ExecuteLocalActivity(disconnectedCtx, s.sentryActivities.ReportPanic, input).Get(disconnectedCtx, nil)

			panic(r)
		}
	}
}
