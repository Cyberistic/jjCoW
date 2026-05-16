version := `cat VERSION 2>/dev/null || echo "dev"`
ldflags := "-X github.com/aranw/jjw/internal/commands.Version=" + version

build:
    go build -ldflags "{{ldflags}}" -o bin/jjw ./cmd/jjw

install:
    go install -ldflags "{{ldflags}}" ./cmd/jjw

clean:
    rm -rf bin/

test:
    go test ./...

fmt:
    go fmt ./...

lint:
    go vet ./...
