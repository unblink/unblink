.PHONY: install proto proto-go proto-ts run-server dev drop-schema

# Install dependencies
install:
	cd app && bun install
	go mod download

# Vendor dependencies
vendor:
	go mod tidy
	go mod vendor

# Generate code from proto files
proto: 
	rm -rf app/gen
	cd proto && buf generate --template buf.gen.ts.yaml
	rm -rf server/gen
	cd proto && buf generate --template buf.gen.go.yaml

# Drop database schema
drop:
	go run cmd/cli/main.go drop

# Typecheck (ts and go)
typecheck:
	cd app && bunx tsc --noEmit
	go vet ./...

delete-app-dir:
	go run cmd/cli/main.go delete-app-dir