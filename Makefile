.PHONY: test test-backend test-agent run-backend run-agent release-agent

test: test-backend test-agent

test-backend:
	cd backend && go test ./...

test-agent:
	cd agent && go test ./...

run-backend:
	cd backend && go run ./cmd/server

run-agent:
	cd agent && go run ./cmd/tunnel-agent version

release-agent:
	powershell -ExecutionPolicy Bypass -File scripts/build-agent-release.ps1
