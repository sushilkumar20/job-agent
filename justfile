# Load .env from the project root into every recipe's environment
set dotenv-load := true

pkg := "./src"

# List available recipes
default:
    @just --list

# Run the agent — pass a prompt, or omit for the default
run *ARGS: env
    go run {{pkg}} {{ARGS}}

# Compile every package without running
build:
    go build ./...

# Build a binary into bin/
binary:
    go build -o bin/jobagent {{pkg}}

# Run tests
test:
    go test ./...

# Format all Go files
fmt:
    go fmt ./...

# Report suspicious constructs
vet:
    go vet ./...

# Sync go.mod/go.sum with imports
tidy:
    go mod tidy

# Show which env vars just loaded (values masked)
env:
    @sed 's/=.*/=<set>/' .env

# Everything CI would check
check: fmt vet build test
