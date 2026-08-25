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
// Note: workflow errors and panics are reported inline from the workflow
// goroutine, so Sentry must be configured with an asynchronous transport (the
// default). A synchronous transport would block the workflow task on network
// I/O and trip the SDK's deadlock detector (1s).
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
