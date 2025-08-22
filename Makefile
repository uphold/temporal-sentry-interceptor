.PHONY: help
help: ## prints help message
	@grep -E '^[a-zA-Z@_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: install
install: ## installs go packages
	go mod tidy

.PHONY: format
format:  ## formats the code using gofumpt
	gofumpt -l -w -extra .

.PHONY: lint
lint:  ## runs linter checks
	golangci-lint run -v --concurrency 4 $(LINT_ARGS)

.PHONY: lint-workflows
lint-workflows: ## runs workflow linting checks
	workflowcheck ./...

.PHONY: lint-fix
lint-fix:  ## runs linter checks with autofix
	$(MAKE) lint LINT_ARGS="--fix"

.PHONY: test
test:  ## runs tests
	go test -timeout 15s ./...

.PHONY: tools
tools: ## install tools (linters, formatters)
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	go install go.temporal.io/sdk/contrib/tools/workflowcheck@latest
	go install mvdan.cc/gofumpt@latest
