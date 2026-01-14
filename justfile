# Ion SFU Sample - Task Runner
# https://github.com/casey/just

# Default recipe to display help
default:
    @just --list

# Start Ion SFU server
sfu:
    docker compose up -d

# Stop Ion SFU server
sfu-down:
    docker compose down

# View SFU logs
sfu-logs:
    docker compose logs -f

# Run publisher (Go client that broadcasts camera/mic)
publish addr="localhost:7000":
    cd publisher && go run main.go -addr {{addr}}

# Build publisher binary
build:
    cd publisher && go build -o broadcaster main.go

# Serve viewer using Ion SDK on specified port
serve-sdk port="8080":
    @echo "Open http://localhost:{{port}}?sfu=ws://localhost:7000/ws in your browser"
    cd viewer-sdk && python3 -m http.server {{port}}

# Serve viewer using raw WebRTC on specified port
serve-raw port="8081":
    @echo "Open http://localhost:{{port}}?sfu=ws://localhost:7000/ws in your browser"
    cd viewer-raw && python3 -m http.server {{port}}

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
