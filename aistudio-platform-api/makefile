run:
	go run main.go

run.dev:
	gow run .

run.track:
	SERVICE_NAME=platformapi INSECURE_MODE=false OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318" go run main.go

test:
	go test -v ./...

testcoverage:
	go test ./... -coverprofile=c.out -covermode=count
	go tool cover -html=c.out -o coverage.html