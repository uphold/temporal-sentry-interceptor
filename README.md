# Temporal Sentry Interceptor

A friendly Go library that seamlessly integrates Sentry error tracking with Temporal workflows and activities.

## Why This Exists

At Uphold, we rely heavily on Temporal for our critical business workflows. When things go wrong (and they sometimes do!), we needed a clean way to capture errors and panics in Sentry for better observability and debugging. After building this solution for our internal projects, we realized the Go ecosystem was missing a package that really nailed this integration, so we decided to open source this!

## Installation

```bash
go get github.com/uphold/temporal-sentry-interceptor
```

## Quick Start

```go
package main

import (
	"log"

	"github.com/getsentry/sentry-go"
	temporalsentry "github.com/uphold/temporal-sentry-interceptor"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"
)

func main() {
	// Initialize Sentry.
	err := sentry.Init(sentry.ClientOptions{
		Dsn: "your-sentry-dsn",
	})
	if err != nil {
		log.Fatalf("Failed to initialize Sentry: %v", err)
	}

	// Create Temporal client.
	c, err := client.Dial(client.Options{
		/* client options */
	})
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	// Create worker with Sentry interceptor.
	w := worker.New(c, "your-task-queue", worker.Options{
		Interceptors: []interceptor.WorkerInterceptor{
			temporalsentry.New(),
		},
	})

	// Register your workflows and activities.
	w.RegisterWorkflow(YourWorkflow)
	w.RegisterActivity(YourActivity)

	// Start the worker.
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}
```

## Configuration Options

### Custom Sentry Scope

Configure how Sentry tags are set:

```go
interceptor := temporalsentry.New(
    temporalsentry.WithConfigureSentryScope(func(
        ctx context.Context,
        request []any,
        activityInfo *activity.Info,
        workflowInfo *workflow.Info,
    ) func(scope *sentry.Scope) {
        return func(scope *sentry.Scope) {
            if workflowInfo != nil {
                scope.SetTag("workflow_type", workflowInfo.WorkflowType.Name)
                scope.SetTag("workflow_id", workflowInfo.WorkflowExecution.ID)
            }
            if activityInfo != nil {
                scope.SetTag("activity_type", activityInfo.ActivityType.Name)
            }
        }
    }),
)
```

### Error and Panic Filtering

Control which errors and panics get reported:

```go
interceptor := temporalsentry.New(
    // Filter workflow errors.
    temporalsentry.WithFilterWorkflowError(func(err error, request []any, info *workflow.Info) bool {
        // Return true to skip reporting this error.
        return errors.Is(err, temporal.ErrCanceled)
    }),

    // Filter workflow panics.
    temporalsentry.WithFilterWorkflowPanic(func(p any, request []any, info *workflow.Info) bool {
        // Return true to skip reporting this panic.
        if panicMsg, ok := p.(string); ok {
            return strings.Contains(panicMsg, "expected panic")
        }
        return false
    }),
)
```

### Custom Activity Options

Configure the activity options for workflow errors and panics:

**Important note**: Local activities should not take more than the Temporal workflow task timeout (which defaults to 10 seconds), be mindful of that if changing the timeout from its default of 5 seconds. [See the difference between Temporal activities and local activities for more information](https://community.temporal.io/t/local-activity-vs-activity/290/3).

```go
interceptor := temporalsentry.New(
    temporalsentry.WithWorkflowErrorActivityOptions(workflow.LocalActivityOptions{
        ScheduleToCloseTimeout: 10 * time.Second,
        RetryPolicy: &temporal.RetryPolicy{
            MaximumAttempts: 3,
        },
    }),
)
```

## Advanced Usage

### Multiple Interceptors

You can easily chain multiple interceptors:

**Important note**: A general rule of thumb is that the Sentry interceptor should go last, as it ensures that it wraps the entire call stack and that other interceptors that might handle errors or recover from panics don't erase them.

```go
w := worker.New(c, "task-queue", worker.Options{
    Interceptors: []interceptor.WorkerInterceptor{
        yourCustomInterceptor.New(),
        temporalsentry.New(/* your options */), // important to place the Sentry interceptor last!
    },
})
```

## Note on Sentry configuration

It is recommended that Sentry's `HTTPSyncTransport` is not used, as all calls to `sentry.CaptureException` and `sentry.Recover` will block on the request being captured to Sentry if that transport is used. If the application's network connection to Sentry's servers is unreliable or unavailable it will cause issues due to the constraints that Temporal's local activities have of not being able to run for more than the amount of time a workflow task can (default is 10 seconds).

## Contributing

We welcome contributions! Whether it's bug reports, feature requests, or pull requests!

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push fork feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- **Issues**: Found a bug? [Open an issue](https://github.com/uphold/temporal-sentry-interceptor/issues)
- **Feature Requests**: Have an idea? We'd love to hear it!

---

Made with ❤️ by the team at [Uphold](https://uphold.com) <img src="https://cdn.uphold.com/images/logo.jpg" width="16px" height="16px" alt="uphold logo">
