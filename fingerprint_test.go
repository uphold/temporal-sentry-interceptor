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

// capturingTransport keeps the events a Sentry client would have sent.
type capturingTransport struct{ events []*sentry.Event }

func (t *capturingTransport) Configure(sentry.ClientOptions)        {}
func (t *capturingTransport) SendEvent(event *sentry.Event)         { t.events = append(t.events, event) }
func (t *capturingTransport) Close()                                {}
func (t *capturingTransport) Flush(time.Duration) bool              { return true }
func (t *capturingTransport) FlushWithContext(context.Context) bool { return true }

// bindCapturingClient points the current hub at a client that records events instead of sending them.
func bindCapturingClient(t *testing.T) *capturingTransport {
	t.Helper()

	transport := &capturingTransport{}
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:       "https://key@example.com/1",
		Transport: transport,
	})
	require.NoError(t, err)

	previous := sentry.CurrentHub().Client()
	sentry.CurrentHub().BindClient(client)
	t.Cleanup(func() { sentry.CurrentHub().BindClient(previous) })

	return transport
}

func activityInfo(name string) *activity.Info {
	return &activity.Info{ActivityType: activity.Type{Name: name}}
}

func workflowInfo(name string) *workflow.Info {
	return &workflow.Info{WorkflowType: workflow.Type{Name: name}}
}

func TestReportErrorInputFingerprint(t *testing.T) {
	tests := []struct {
		name     string
		input    ReportErrorInput
		expected []string
	}{
		{
			name: "activity error uses the activity type and the error type",
			input: ReportErrorInput{
				Error:        temporal.NewApplicationError("quote denied", "forbidden"),
				EventName:    "ActivityExecuteActivity",
				ActivityInfo: activityInfo("CreateQuote"),
			},
			expected: []string{"ActivityExecuteActivity", "CreateQuote", "forbidden"},
		},
		{
			name: "workflow error uses the workflow type",
			input: ReportErrorInput{
				Error:        temporal.NewApplicationError("setup failed", "internal"),
				EventName:    "WorkflowExecuteWorkflow",
				WorkflowInfo: workflowInfo("SetupTransaction"),
			},
			expected: []string{"WorkflowExecuteWorkflow", "SetupTransaction", "internal"},
		},
		{
			name: "falls back to the Go type when the error is not an application error",
			input: ReportErrorInput{
				Error:        errors.New("boom"),
				EventName:    "ActivityExecuteActivity",
				ActivityInfo: activityInfo("CreateQuote"),
			},
			expected: []string{"ActivityExecuteActivity", "CreateQuote", "*errors.errorString"},
		},
		{
			name: "falls back to the Go type when the application error has no type",
			input: ReportErrorInput{
				Error:        temporal.NewApplicationError("boom", ""),
				EventName:    "ActivityExecuteActivity",
				ActivityInfo: activityInfo("CreateQuote"),
			},
			// The SDK aliases temporal.ApplicationError to its internal package, which is what %T prints.
			expected: []string{"ActivityExecuteActivity", "CreateQuote", "*internal.ApplicationError"},
		},
		{
			name: "omits the name when neither info is set",
			input: ReportErrorInput{
				Error:     temporal.NewApplicationError("boom", "forbidden"),
				EventName: "ActivityExecuteActivity",
			},
			expected: []string{"ActivityExecuteActivity", "forbidden"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.input.Fingerprint())
		})
	}
}

func TestReportErrorFingerprintSeparatesFailures(t *testing.T) {
	t.Run("same activity, different error types", func(t *testing.T) {
		forbidden := ReportErrorInput{
			Error:        temporal.NewApplicationError("denied", "forbidden"),
			EventName:    "ActivityExecuteActivity",
			ActivityInfo: activityInfo("CreateQuote"),
		}
		conflict := ReportErrorInput{
			Error:        temporal.NewApplicationError("duplicate", "conflict"),
			EventName:    "ActivityExecuteActivity",
			ActivityInfo: activityInfo("CreateQuote"),
		}

		assert.NotEqual(t, forbidden.Fingerprint(), conflict.Fingerprint())
	})

	t.Run("same error type, different activities", func(t *testing.T) {
		quote := ReportErrorInput{
			Error:        temporal.NewApplicationError("denied", "forbidden"),
			EventName:    "ActivityExecuteActivity",
			ActivityInfo: activityInfo("CreateQuote"),
		}
		schedule := ReportErrorInput{
			Error:        temporal.NewApplicationError("denied", "forbidden"),
			EventName:    "ActivityExecuteActivity",
			ActivityInfo: activityInfo("CreateSchedule"),
		}

		assert.NotEqual(t, quote.Fingerprint(), schedule.Fingerprint())
	})
}

func TestReportErrorAppliesFingerprintToTheEvent(t *testing.T) {
	t.Run("sets the default fingerprint", func(t *testing.T) {
		transport := bindCapturingClient(t)
		activities := &sentryActivities{}

		err := activities.ReportError(context.Background(), ReportErrorInput{
			Error:        temporal.NewApplicationError("quote denied", "forbidden"),
			EventName:    "ActivityExecuteActivity",
			ActivityInfo: activityInfo("CreateQuote"),
		})
		require.NoError(t, err)

		require.Len(t, transport.events, 1)
		assert.Equal(
			t,
			[]string{"ActivityExecuteActivity", "CreateQuote", "forbidden"},
			transport.events[0].Fingerprint,
		)
	})

	t.Run("lets a custom scope override it", func(t *testing.T) {
		transport := bindCapturingClient(t)
		activities := &sentryActivities{
			configureSentryScope: func(
				_ context.Context, _ []any, _ string, _ *activity.Info, _ *workflow.Info,
			) func(scope *sentry.Scope) {
				return func(scope *sentry.Scope) {
					scope.SetFingerprint([]string{"custom"})
				}
			},
		}

		err := activities.ReportError(context.Background(), ReportErrorInput{
			Error:        temporal.NewApplicationError("quote denied", "forbidden"),
			EventName:    "ActivityExecuteActivity",
			ActivityInfo: activityInfo("CreateQuote"),
		})
		require.NoError(t, err)

		require.Len(t, transport.events, 1)
		assert.Equal(t, []string{"custom"}, transport.events[0].Fingerprint)
	})

	t.Run("leaves panics on the default grouping", func(t *testing.T) {
		transport := bindCapturingClient(t)
		activities := &sentryActivities{}

		err := activities.ReportPanic(context.Background(), ReportPanicInput{
			Panic:        "boom",
			EventName:    "ActivityExecuteActivityPanic",
			ActivityInfo: activityInfo("CreateQuote"),
		})
		require.NoError(t, err)

		require.Len(t, transport.events, 1)
		assert.Empty(t, transport.events[0].Fingerprint)
	})
}
