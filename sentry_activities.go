package temporalsentryinterceptor

import (
	"context"
	"errors"
	"fmt"

	"github.com/getsentry/sentry-go"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
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

// Fingerprint returns the Sentry grouping key for the reported error.
//
// Errors are captured here rather than where they happen, so every event carries this
// interceptor's stack trace. Sentry groups on that stack by default, which bundles unrelated
// failures into a single issue. Grouping on the event, the workflow or activity that failed and
// the error type instead keeps each failure mode on its own.
func (i ReportErrorInput) Fingerprint() []string {
	fingerprint := []string{i.EventName}

	switch {
	case i.ActivityInfo != nil:
		fingerprint = append(fingerprint, i.ActivityInfo.ActivityType.Name)
	case i.WorkflowInfo != nil:
		fingerprint = append(fingerprint, i.WorkflowInfo.WorkflowType.Name)
	}

	var applicationError *temporal.ApplicationError
	if errors.As(i.Error, &applicationError) && applicationError.Type() != "" {
		return append(fingerprint, applicationError.Type())
	}

	return append(fingerprint, fmt.Sprintf("%T", i.Error))
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

	// Applied before the configured scope so that it can still override the fingerprint.
	hub.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetFingerprint(input.Fingerprint())
	})

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
