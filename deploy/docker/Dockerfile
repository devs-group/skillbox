# ============================================================
# Stage 1: Build
# ============================================================
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache ca-certificates tzdata git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -ldflags="-w -s -X github.com/devs-group/skillbox/internal/version.Version=$(git describe --tags --always 2>/dev/null || echo dev) -X github.com/devs-group/skillbox/internal/version.Commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)" \
      -trimpath \
      -o /bin/skillbox-server \
      ./cmd/skillbox-server

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -ldflags="-w -s" \
      -trimpath \
      -o /bin/skillbox \
      ./cmd/skillbox

# ============================================================
# Stage 2: Runtime
# ============================================================
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/skillbox-server /skillbox-server
COPY --from=builder /bin/skillbox /skillbox

USER nonroot:nonroot

EXPOSE 8080 9090

ENTRYPOINT ["/skillbox-server"]
