package temporalsentryinterceptor

import (
	"go.temporal.io/sdk/interceptor"
)

// TemporalWorkerInterceptor provides Sentry error and panic reporting for Temporal workflows and activities.
type TemporalWorkerInterceptor struct {
	options          *options
	sentryActivities sentryActivities
	*interceptor.WorkerInterceptorBase
}

// New creates a new TemporalWorkerInterceptor with the provided options.
//
// Note: This interceptor uses local activities for error reporting which must
// complete within the workflow task timeout (default 10s).
// The default timeout is 5s to provide a safety margin.
// Learn more about local activities vs activities here:
// https://community.temporal.io/t/local-activity-vs-activity/290/3.
func New(opts ...Option) *TemporalWorkerInterceptor {
	interceptor := &TemporalWorkerInterceptor{
		options: defaultOptions(),
	}

	for _, fn := range opts {
		fn(interceptor.options)
	}

	interceptor.sentryActivities = sentryActivities{
		configureSentryScope: interceptor.options.configureSentryScope,
	}

	return interceptor
}
