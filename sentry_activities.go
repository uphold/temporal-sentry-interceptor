package temporalsentryinterceptor

import (
	"context"

	"github.com/getsentry/sentry-go"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
)

// sentryActivities handles Sentry error and panic reporting for Temporal workflows and activities.
type sentryActivities struct {
	configureSentryScope ConfigureSentryScopeFunc
}

// ReportErrorInput contains data needed to report an error to Sentry.
type ReportErrorInput struct {
	Error        error
	EventName    string
	Request      []any
	ActivityInfo *activity.Info
	WorkflowInfo *workflow.Info
}

// ReportPanicInput contains data needed to report a panic to Sentry.
type ReportPanicInput struct {
	Panic        any
	EventName    string
	Request      []any
	ActivityInfo *activity.Info
	WorkflowInfo *workflow.Info
}

// ReportError reports an error to Sentry with configured scope and context.
func (s *sentryActivities) ReportError(ctx context.Context, input ReportErrorInput) error {
	if sentry.CurrentHub() == nil {
		return nil
	}

	hub := sentry.CurrentHub().Clone()
	if s.configureSentryScope != nil {
		scopeFunction := s.configureSentryScope(ctx, input.Request, input.EventName, input.ActivityInfo, input.WorkflowInfo)
		hub.ConfigureScope(scopeFunction)
	}

	hub.CaptureException(input.Error)

	return nil
}

// ReportPanic reports a panic to Sentry with configured scope and context.
func (s *sentryActivities) ReportPanic(ctx context.Context, input ReportPanicInput) error {
	if sentry.CurrentHub() == nil {
		return nil
	}

	hub := sentry.CurrentHub().Clone()
	if s.configureSentryScope != nil {
		scopeFunction := s.configureSentryScope(ctx, input.Request, input.EventName, input.ActivityInfo, input.WorkflowInfo)
		hub.ConfigureScope(scopeFunction)
	}

	hub.RecoverWithContext(ctx, input.Panic)

	return nil
}
