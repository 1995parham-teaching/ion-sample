# LiveKit Sample - Task Runner
# https://github.com/casey/just

# Default recipe to display help
default:
    @just --list

# Start LiveKit server
sfu:
    docker compose up -d

# Stop LiveKit server
sfu-down:
    docker compose down

# View LiveKit logs
sfu-logs:
    docker compose logs -f

# Run publisher (Go client that broadcasts camera/mic)
publish host="http://localhost:7880" api_key="devkey" api_secret="secret":
    cd publisher && go run main.go -host {{host}} -api-key {{api_key}} -api-secret {{api_secret}}

# Build publisher binary
build:
    cd publisher && go build -o broadcaster main.go

# Serve viewer on specified port
serve port="8080":
    @echo "Open http://localhost:{{port}} in your browser"
    cd viewer && python3 -m http.server {{port}}

# Update Go dependencies
update-deps:
    cd publisher && go get -u ./... && go mod tidy

# Format Go code
fmt:
    cd publisher && go fmt ./...

# Run Go vet
vet:
    cd publisher && go vet ./...

# Clean build artifacts
clean:
    rm -f publisher/broadcaster
