# Build stage
FROM golang:1.22 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/punchtrunk ./cmd/punchtrunk

# Runtime: distroless static
FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=build /out/punchtrunk /app/punchtrunk
USER nonroot:nonroot
ENTRYPOINT ["/app/punchtrunk"]
