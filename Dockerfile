# Build stage
FROM golang:1.22 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/trunk-orchestrator ./cmd/trunk-orchestrator

# Runtime: distroless static
FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=build /out/trunk-orchestrator /app/trunk-orchestrator
USER nonroot:nonroot
ENTRYPOINT ["/app/trunk-orchestrator"]
