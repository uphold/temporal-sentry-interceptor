package temporalsentryinterceptor

import (
	"context"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/interceptor"
)

// TemporalActivityInterceptor provides Sentry error and panic reporting for Temporal activity executions.
type TemporalActivityInterceptor struct {
	sentryActivities    sentryActivities
	filterActivityError FilterActivityErrorFunc
	filterActivityPanic FilterActivityPanicFunc
	interceptor.ActivityInboundInterceptorBase
}

// InterceptActivity creates and returns an activity interceptor with Sentry integration.
func (s TemporalWorkerInterceptor) InterceptActivity(
	_ context.Context, next interceptor.ActivityInboundInterceptor,
) interceptor.ActivityInboundInterceptor {
	return &TemporalActivityInterceptor{
		sentryActivities:    s.sentryActivities,
		filterActivityError: s.options.filterActivityError,
		filterActivityPanic: s.options.filterActivityPanic,
		ActivityInboundInterceptorBase: interceptor.ActivityInboundInterceptorBase{
			Next: next,
		},
	}
}

// ExecuteActivity intercepts activity execution and reports errors/panics to Sentry.
func (s *TemporalActivityInterceptor) ExecuteActivity(
	ctx context.Context, in *interceptor.ExecuteActivityInput,
) (any, error) {
	info := activity.GetInfo(ctx)
	defer s.captureActivityPanicToSentry(ctx, "ActivityExecuteActivityPanic", in.Args, &info)()

	result, err := s.Next.ExecuteActivity(ctx, in)
	if err != nil {
		s.captureActivityErrorToSentry(ctx, err, "ActivityExecuteActivity", in.Args, &info)
	}

	return result, err
}

// captureActivityErrorToSentry reports activity errors to Sentry with context and filtering.
func (s *TemporalActivityInterceptor) captureActivityErrorToSentry(
	ctx context.Context, err error, eventName string, req []any, info *activity.Info,
) {
	if s.filterActivityError != nil && s.filterActivityError(err, req, info) {
		return
	}

	input := ReportErrorInput{Error: err, EventName: eventName, Request: req, ActivityInfo: info}
	_ = s.sentryActivities.ReportError(ctx, input)
}

// captureActivityPanicToSentry returns a deferred function that reports activity panics to Sentry.
func (s *TemporalActivityInterceptor) captureActivityPanicToSentry(
	ctx context.Context, eventName string, req []any, info *activity.Info,
) func() {
	return func() {
		if r := recover(); r != nil {
			if s.filterActivityPanic != nil && s.filterActivityPanic(r, req, info) {
				return
			}

			input := ReportPanicInput{Panic: r, EventName: eventName, Request: req, ActivityInfo: info}
			_ = s.sentryActivities.ReportPanic(ctx, input)

			panic(r)
		}
	}
}
