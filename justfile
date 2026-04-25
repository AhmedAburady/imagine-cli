# Git commands
mod git


# Install Dev
install:
  go install -ldflags "-X main.version=dev" ./cmd/imagine

# Build binary
build:
  go build -ldflags "-X main.version=dev" -o imagine ./cmd/imagine 2>&1