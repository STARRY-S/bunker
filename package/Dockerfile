
FROM registry.suse.com/bci/golang:1.24 AS build

WORKDIR /app

# Pre-download dependencies
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=bind,source=go.mod,target=go.mod \
    go mod download -x

# Build the binary
ENV GOCACHE=/root/.cache/go-build
ARG TAG=v0.1.0-alpha
ARG COMMIT=unknown
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=cache,target="/root/.cache/go-build" \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 \
    go build \
    -buildmode=pie \
    -ldflags="-extldflags='-static' -s -w -X github.com/STARRY-S/bunker/pkg/utils.Version=${TAG} -X github.com/STARRY-S/bunker/pkg/utils.Commit=${COMMIT}" \
    -o /usr/local/bin/bunker \
    .
RUN bunker version

FROM registry.suse.com/bci/bci-base:15.7 AS server

COPY --from=build /usr/local/bin/bunker /usr/local/bin/
COPY config.example.yaml /config.yaml

# Run Bunker Reverse Proxy Server
ENTRYPOINT [ "bunker" ]
CMD [ "run", "-c=/config.yaml" ]
