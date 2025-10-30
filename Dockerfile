# Build stage
FROM golang:1.25 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/punchtrunk ./cmd/punchtrunk

# Download Trunk CLI (version from .trunk/trunk.yaml)
FROM alpine:3.19 AS trunkcli
WORKDIR /trunk
SHELL ["/bin/ash", "-eo", "pipefail", "-c"]
COPY .trunk/trunk.yaml ./trunk.yaml
# Install curl, then extract cli.version and download Trunk CLI
RUN apk add --no-cache curl >/dev/null; \
	TRUNK_VERSION=$(awk '/^cli:/ {found=1} found && /version:/ {print $2; exit}' trunk.yaml); \
	if [ -z "$TRUNK_VERSION" ]; then echo "Could not parse cli.version from trunk.yaml"; exit 1; fi; \
	curl -Ls "https://trunk.io/releases/${TRUNK_VERSION}/trunk-${TRUNK_VERSION}-linux-x86_64.tar.gz" | tar -xz; \
	chmod +x trunk

# Runtime: distroless static
FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=build /out/punchtrunk /app/punchtrunk
COPY --from=trunkcli /trunk/trunk /app/trunk
ENV PUNCHTRUNK_TRUNK_BINARY=/app/trunk
USER nonroot:nonroot
ENTRYPOINT ["/app/punchtrunk"]
