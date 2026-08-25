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

## Issue Grouping

Errors are captured from the interceptor rather than from the code that failed, so every event
carries the same stack trace. Sentry groups on that stack by default, which bundles unrelated
failures into one ever-growing issue — the title then describes whichever error happened to fire
last, and the issue reopens forever because something in the bundle keeps failing.

To avoid that, errors are reported with an explicit fingerprint:

```
<event name> / <workflow or activity type> / <error type>
```

The error type comes from `temporal.ApplicationError.Type()`, falling back to the Go type of the
error. Both are low cardinality and stable, so each failure mode gets its own issue.

Panics are left on Sentry's default grouping: a recovered panic carries a real stack trace, so
grouping already works for them.

To group differently, set your own fingerprint from `WithConfigureSentryScope` — it runs after the
default and overrides it:

```go
temporalsentry.WithConfigureSentryScope(func(
    ctx context.Context,
    request []any,
    eventName string,
    activityInfo *activity.Info,
    workflowInfo *workflow.Info,
) func(scope *sentry.Scope) {
    return func(scope *sentry.Scope) {
        scope.SetFingerprint([]string{"my-own-key"})
    }
})
```

## Configuration Options

### Custom Sentry Scope

Configure how Sentry tags are set:

```go
interceptor := temporalsentry.New(
    temporalsentry.WithConfigureSentryScope(func(
        ctx context.Context,
        request []any,
        eventName string,
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

Sentry's `HTTPSyncTransport` must not be used. Workflow errors and panics are reported inline from the workflow goroutine — with an asynchronous transport (Sentry's default) `sentry.CaptureException` and `sentry.Recover` only enqueue the event and return immediately, but a synchronous transport would block the workflow task on network I/O and trip the Temporal SDK's deadlock detector (1 second).

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
