package temporalsentryinterceptor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
	}{
		{
			name: "default options",
			opts: nil,
		},
		{
			name: "with custom sentry scope",
			opts: []Option{
				WithConfigureSentryScope(func(
					_ context.Context, _ []any, _ string, _ *activity.Info, _ *workflow.Info,
				) func(scope *sentry.Scope) {
					return func(scope *sentry.Scope) {
						scope.SetTag("test", "true")
					}
				}),
			},
		},
		{
			name: "with filters",
			opts: []Option{
				WithFilterWorkflowError(func(err error, _ []any, _ *workflow.Info) bool {
					return temporal.IsCanceledError(err)
				}),
				WithFilterWorkflowPanic(func(p any, _ []any, _ *workflow.Info) bool {
					return p == "test panic"
				}),
			},
		},
		{
			name: "with activity options",
			opts: []Option{
				WithWorkflowErrorActivityOptions(workflow.LocalActivityOptions{
					ScheduleToCloseTimeout: 10 * time.Second,
				}),
				WithWorkflowPanicActivityOptions(workflow.LocalActivityOptions{
					ScheduleToCloseTimeout: 15 * time.Second,
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interceptor := New(tt.opts...)
			assert.NotNil(t, interceptor)
			assert.NotNil(t, interceptor.options)
			assert.NotNil(t, interceptor.sentryActivities)
		})
	}
}

func TestOptions(t *testing.T) {
	t.Run("WithConfigureSentryScope", func(t *testing.T) {
		called := false
		scopeFunc := func(
			_ context.Context, _ []any, _ string, _ *activity.Info, _ *workflow.Info,
		) func(_ *sentry.Scope) {
			called = true
			return func(_ *sentry.Scope) {}
		}

		interceptor := New(WithConfigureSentryScope(scopeFunc))
		assert.NotNil(t, interceptor.options.configureSentryScope)

		interceptor.options.configureSentryScope(context.Background(), nil, "", nil, nil)
		assert.True(t, called)
	})

	t.Run("WithFilterWorkflowError", func(t *testing.T) {
		testError := errors.New("test error")
		filterFunc := func(err error, _ []any, _ *workflow.Info) bool {
			return errors.Is(err, testError)
		}

		interceptor := New(WithFilterWorkflowError(filterFunc))
		assert.NotNil(t, interceptor.options.filterWorkflowError)

		result := interceptor.options.filterWorkflowError(testError, nil, nil)
		assert.True(t, result)
	})

	t.Run("WithFilterWorkflowPanic", func(t *testing.T) {
		filterFunc := func(p any, _ []any, _ *workflow.Info) bool {
			return p == "filtered panic"
		}

		interceptor := New(WithFilterWorkflowPanic(filterFunc))
		assert.NotNil(t, interceptor.options.filterWorkflowPanic)

		result := interceptor.options.filterWorkflowPanic("filtered panic", nil, nil)
		assert.True(t, result)

		result = interceptor.options.filterWorkflowPanic("other panic", nil, nil)
		assert.False(t, result)
	})

	t.Run("WithFilterActivityError", func(t *testing.T) {
		testError := errors.New("activity error")
		filterFunc := func(err error, _ []any, _ *activity.Info) bool {
			return errors.Is(err, testError)
		}

		interceptor := New(WithFilterActivityError(filterFunc))
		assert.NotNil(t, interceptor.options.filterActivityError)

		result := interceptor.options.filterActivityError(testError, nil, nil)
		assert.True(t, result)
	})

	t.Run("WithFilterActivityPanic", func(t *testing.T) {
		filterFunc := func(p any, _ []any, _ *activity.Info) bool {
			return p == "activity panic"
		}

		interceptor := New(WithFilterActivityPanic(filterFunc))
		assert.NotNil(t, interceptor.options.filterActivityPanic)

		result := interceptor.options.filterActivityPanic("activity panic", nil, nil)
		assert.True(t, result)
	})

	t.Run("WithWorkflowPanicActivityOptions", func(t *testing.T) {
		opts := workflow.LocalActivityOptions{
			ScheduleToCloseTimeout: 30 * time.Second,
		}

		interceptor := New(WithWorkflowPanicActivityOptions(opts))
		assert.Equal(t, opts, interceptor.options.workflowPanicActivityOptions)
	})

	t.Run("WithWorkflowErrorActivityOptions", func(t *testing.T) {
		opts := workflow.LocalActivityOptions{
			ScheduleToCloseTimeout: 25 * time.Second,
		}

		interceptor := New(WithWorkflowErrorActivityOptions(opts))
		assert.Equal(t, opts, interceptor.options.workflowErrorActivityOptions)
	})
}

func TestDefaultOptions(t *testing.T) {
	opts := defaultOptions()

	assert.NotNil(t, opts.configureSentryScope)
	assert.Nil(t, opts.filterWorkflowError)
	assert.Nil(t, opts.filterWorkflowPanic)
	assert.Nil(t, opts.filterActivityError)
	assert.Nil(t, opts.filterActivityPanic)

	assert.Equal(t, "ReportPanicToSentry", opts.workflowPanicActivityOptions.Summary)
	assert.Equal(t, 5*time.Second, opts.workflowPanicActivityOptions.ScheduleToCloseTimeout)
	assert.Equal(t, int32(1), opts.workflowPanicActivityOptions.RetryPolicy.MaximumAttempts)

	assert.Equal(t, "ReportErrorToSentry", opts.workflowErrorActivityOptions.Summary)
	assert.Equal(t, 5*time.Second, opts.workflowErrorActivityOptions.ScheduleToCloseTimeout)
	assert.Equal(t, int32(1), opts.workflowErrorActivityOptions.RetryPolicy.MaximumAttempts)
}

func TestSentryActivitiesReportError(t *testing.T) {
	t.Run("successful error reporting", func(t *testing.T) {
		activities := &sentryActivities{}
		input := ReportErrorInput{
			Error:     errors.New("test error"),
			EventName: "TestEvent",
			Request:   []any{"test"},
		}

		err := activities.ReportError(context.Background(), input)
		assert.NoError(t, err)
	})

	t.Run("with configure scope function", func(t *testing.T) {
		scopeConfigured := false
		activities := &sentryActivities{
			configureSentryScope: func(
				_ context.Context, _ []any, _ string, _ *activity.Info, _ *workflow.Info,
			) func(scope *sentry.Scope) {
				scopeConfigured = true
				return func(scope *sentry.Scope) {
					scope.SetTag("test", "true")
				}
			},
		}

		input := ReportErrorInput{
			Error:     errors.New("test error"),
			EventName: "TestEvent",
			Request:   []any{"test"},
		}

		err := activities.ReportError(context.Background(), input)
		require.NoError(t, err)
		assert.True(t, scopeConfigured)
	})
}

func TestSentryActivitiesReportPanic(t *testing.T) {
	t.Run("successful panic reporting", func(t *testing.T) {
		activities := &sentryActivities{}
		input := ReportPanicInput{
			Panic:     "test panic",
			EventName: "TestPanic",
			Request:   []any{"test"},
		}

		err := activities.ReportPanic(context.Background(), input)
		assert.NoError(t, err)
	})

	t.Run("with configure scope function", func(t *testing.T) {
		scopeConfigured := false
		activities := &sentryActivities{
			configureSentryScope: func(
				_ context.Context, _ []any, _ string, _ *activity.Info, _ *workflow.Info,
			) func(scope *sentry.Scope) {
				scopeConfigured = true
				return func(scope *sentry.Scope) {
					scope.SetTag("panic", "true")
				}
			},
		}

		input := ReportPanicInput{
			Panic:     "test panic",
			EventName: "TestPanic",
			Request:   []any{"test"},
		}

		err := activities.ReportPanic(context.Background(), input)
		require.NoError(t, err)
		assert.True(t, scopeConfigured)
	})
}

func TestInterceptorIntegration(t *testing.T) {
	t.Run("complete interceptor setup", func(t *testing.T) {
		scopeConfigured := false
		errorFiltered := false
		panicFiltered := false

		interceptor := New(
			WithConfigureSentryScope(
				func(
					_ context.Context, _ []any, _ string, _ *activity.Info, _ *workflow.Info,
				) func(scope *sentry.Scope) {
					scopeConfigured = true
					return func(scope *sentry.Scope) {
						scope.SetTag("integration_test", "true")
					}
				}),
			WithFilterWorkflowError(func(_ error, _ []any, _ *workflow.Info) bool {
				errorFiltered = true
				return false
			}),
			WithFilterWorkflowPanic(func(_ any, _ []any, _ *workflow.Info) bool {
				panicFiltered = true
				return false
			}),
			WithWorkflowErrorActivityOptions(workflow.LocalActivityOptions{
				ScheduleToCloseTimeout: 10 * time.Second,
			}),
		)

		assert.NotNil(t, interceptor)
		assert.NotNil(t, interceptor.options)
		assert.NotNil(t, interceptor.sentryActivities)

		assert.NotNil(t, interceptor.options.configureSentryScope)
		assert.NotNil(t, interceptor.options.filterWorkflowError)
		assert.NotNil(t, interceptor.options.filterWorkflowPanic)

		interceptor.options.filterWorkflowError(errors.New("test"), nil, nil)
		assert.True(t, errorFiltered)

		interceptor.options.filterWorkflowPanic("test panic", nil, nil)
		assert.True(t, panicFiltered)

		interceptor.options.configureSentryScope(context.Background(), nil, "", nil, nil)
		assert.True(t, scopeConfigured)
	})
}
