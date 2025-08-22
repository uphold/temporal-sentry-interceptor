package temporalsentryinterceptor

import (
	"context"
	"time"

	"github.com/getsentry/sentry-go"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type (
	// ConfigureSentryScopeFunc defines a function to configure Sentry scope with context and request data.
	ConfigureSentryScopeFunc = func(
		ctx context.Context, request []any, eventName string, activityInfo *activity.Info, workflowInfo *workflow.Info,
	) func(scope *sentry.Scope)
	// FilterWorkflowErrorFunc defines a function to filter workflow errors before reporting to Sentry.
	FilterWorkflowErrorFunc = func(err error, request []any, info *workflow.Info) bool
	// FilterWorkflowPanicFunc defines a function to filter workflow panics before reporting to Sentry.
	FilterWorkflowPanicFunc = func(p any, request []any, info *workflow.Info) bool
	// FilterActivityErrorFunc defines a function to filter activity errors before reporting to Sentry.
	FilterActivityErrorFunc = func(err error, request []any, info *activity.Info) bool
	// FilterActivityPanicFunc defines a function to filter activity panics before reporting to Sentry.
	FilterActivityPanicFunc = func(p any, request []any, info *activity.Info) bool
	// Option defines a configuration option for the interceptor.
	Option = func(*options)
)

// options holds configuration settings for the Temporal Sentry interceptor.
type options struct {
	configureSentryScope         ConfigureSentryScopeFunc
	filterWorkflowError          FilterWorkflowErrorFunc
	filterWorkflowPanic          FilterWorkflowPanicFunc
	filterActivityError          FilterActivityErrorFunc
	filterActivityPanic          FilterActivityPanicFunc
	workflowPanicActivityOptions workflow.LocalActivityOptions
	workflowErrorActivityOptions workflow.LocalActivityOptions
}

// defaultOptions returns default configuration options for the interceptor.
func defaultOptions() *options {
	o := &options{
		configureSentryScope: func(
			_ context.Context, _ []any, eventName string, activityInfo *activity.Info, workflowInfo *workflow.Info,
		) func(scope *sentry.Scope) {
			return func(scope *sentry.Scope) {
				scope.SetTag("temporal.event_type", eventName)

				if workflowInfo != nil {
					scope.SetTag("temporal.workflow_type", workflowInfo.WorkflowType.Name)
					scope.SetExtra("temporal.workflow_id", workflowInfo.WorkflowExecution.ID)
					scope.SetExtra("temporal.workflow_run_id", workflowInfo.WorkflowExecution.RunID)
					scope.SetExtra("temporal.workflow_task_queue", workflowInfo.TaskQueueName)
					scope.SetExtra("temporal.workflow_attempt", workflowInfo.Attempt)
				}

				if activityInfo != nil {
					scope.SetTag("temporal.activity_type", activityInfo.ActivityType.Name)
					scope.SetExtra("temporal.activity_id", activityInfo.ActivityID)
					scope.SetExtra("temporal.activity_task_queue", activityInfo.TaskQueue)
					scope.SetExtra("temporal.activity_attempt", activityInfo.Attempt)
				}
			}
		},
		filterWorkflowError: nil,
		filterWorkflowPanic: nil,
		filterActivityError: nil,
		filterActivityPanic: nil,
		workflowPanicActivityOptions: workflow.LocalActivityOptions{
			Summary:                "ReportPanicToSentry",
			ScheduleToCloseTimeout: 5 * time.Second,
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 1,
			},
		},
		workflowErrorActivityOptions: workflow.LocalActivityOptions{
			Summary:                "ReportErrorToSentry",
			ScheduleToCloseTimeout: 5 * time.Second,
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 1,
			},
		},
	}

	return o
}

// WithConfigureSentryScope sets a custom function to configure Sentry scope.
func WithConfigureSentryScope(configureSentryScope ConfigureSentryScopeFunc) Option {
	return func(o *options) {
		o.configureSentryScope = configureSentryScope
	}
}

// WithFilterWorkflowError sets a filter function for workflow errors.
func WithFilterWorkflowError(filterWorkflowError FilterWorkflowErrorFunc) Option {
	return func(o *options) {
		o.filterWorkflowError = filterWorkflowError
	}
}

// WithFilterWorkflowPanic sets a filter function for workflow panics.
func WithFilterWorkflowPanic(filterWorkflowPanic FilterWorkflowPanicFunc) Option {
	return func(o *options) {
		o.filterWorkflowPanic = filterWorkflowPanic
	}
}

// WithFilterActivityError sets a filter function for activity errors.
func WithFilterActivityError(filterActivityError FilterActivityErrorFunc) Option {
	return func(o *options) {
		o.filterActivityError = filterActivityError
	}
}

// WithFilterActivityPanic sets a filter function for activity panics.
func WithFilterActivityPanic(filterActivityPanic FilterActivityPanicFunc) Option {
	return func(o *options) {
		o.filterActivityPanic = filterActivityPanic
	}
}

// WithWorkflowPanicActivityOptions sets custom activity options for panic reporting.
func WithWorkflowPanicActivityOptions(workflowPanicActivityOptions workflow.LocalActivityOptions) Option {
	return func(o *options) {
		o.workflowPanicActivityOptions = workflowPanicActivityOptions
	}
}

// WithWorkflowErrorActivityOptions sets custom activity options for error reporting.
func WithWorkflowErrorActivityOptions(workflowErrorActivityOptions workflow.LocalActivityOptions) Option {
	return func(o *options) {
		o.workflowErrorActivityOptions = workflowErrorActivityOptions
	}
}
