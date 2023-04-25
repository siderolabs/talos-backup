# syntax = docker/dockerfile-upstream:1.5.2-labs

# THIS FILE WAS AUTOMATICALLY GENERATED, PLEASE DO NOT EDIT.
#
# Generated on 2023-04-25T14:09:23Z by kres latest.

ARG TOOLCHAIN

# cleaned up specs and compiled versions
FROM scratch AS generate

FROM ghcr.io/siderolabs/ca-certificates:v1.4.1 AS image-ca-certificates

FROM ghcr.io/siderolabs/fhs:v1.4.1 AS image-fhs

# runs markdownlint
FROM docker.io/node:20.0.0-alpine3.16 AS lint-markdown
WORKDIR /src
RUN npm i -g markdownlint-cli@0.33.0
RUN npm i sentences-per-line@0.2.1
COPY .markdownlint.json .
COPY ./README.md ./README.md
RUN markdownlint --ignore "CHANGELOG.md" --ignore "**/node_modules/**" --ignore '**/hack/chglog/**' --rules node_modules/sentences-per-line/index.js .

# base toolchain image
FROM ${TOOLCHAIN} AS toolchain
RUN apk --update --no-cache add bash curl build-base protoc protobuf-dev

# build tools
FROM --platform=${BUILDPLATFORM} toolchain AS tools
ENV GO111MODULE on
ARG CGO_ENABLED
ENV CGO_ENABLED ${CGO_ENABLED}
ENV GOPATH /go
ARG GOLANGCILINT_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCILINT_VERSION} \
	&& mv /go/bin/golangci-lint /bin/golangci-lint
ARG GOFUMPT_VERSION
RUN go install mvdan.cc/gofumpt@${GOFUMPT_VERSION} \
	&& mv /go/bin/gofumpt /bin/gofumpt
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install golang.org/x/vuln/cmd/govulncheck@latest \
	&& mv /go/bin/govulncheck /bin/govulncheck
ARG GOIMPORTS_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install golang.org/x/tools/cmd/goimports@${GOIMPORTS_VERSION} \
	&& mv /go/bin/goimports /bin/goimports
ARG DEEPCOPY_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install github.com/siderolabs/deep-copy@${DEEPCOPY_VERSION} \
	&& mv /go/bin/deep-copy /bin/deep-copy

# tools and sources
FROM tools AS base
WORKDIR /src
COPY ./go.mod .
COPY ./go.sum .
RUN --mount=type=cache,target=/go/pkg go mod download
RUN --mount=type=cache,target=/go/pkg go mod verify
COPY ./cmd ./cmd
COPY ./internal ./internal
COPY ./pkg ./pkg
RUN --mount=type=cache,target=/go/pkg go list -mod=readonly all >/dev/null

# runs gofumpt
FROM base AS lint-gofumpt
RUN FILES="$(gofumpt -l .)" && test -z "${FILES}" || (echo -e "Source code is not formatted with 'gofumpt -w .':\n${FILES}"; exit 1)

# runs goimports
FROM base AS lint-goimports
RUN FILES="$(goimports -l -local github.com/siderolabs/talos-backup .)" && test -z "${FILES}" || (echo -e "Source code is not formatted with 'goimports -w -local github.com/siderolabs/talos-backup .':\n${FILES}"; exit 1)

# runs golangci-lint
FROM base AS lint-golangci-lint
COPY .golangci.yml .
ENV GOGC 50
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/root/.cache/golangci-lint --mount=type=cache,target=/go/pkg golangci-lint run --config .golangci.yml

# runs govulncheck
FROM base AS lint-govulncheck
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg govulncheck ./...

# builds talos-backup-linux-amd64
FROM base AS talos-backup-linux-amd64-build
COPY --from=generate / /
WORKDIR /src/cmd/talos-backup
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg GOARCH=amd64 GOOS=linux go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /talos-backup-linux-amd64

# builds talos-backup-linux-arm64
FROM base AS talos-backup-linux-arm64-build
COPY --from=generate / /
WORKDIR /src/cmd/talos-backup
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg GOARCH=arm64 GOOS=linux go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /talos-backup-linux-arm64

# runs unit-tests with race detector
FROM base AS unit-tests-race
ARG TESTPKGS
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg --mount=type=cache,target=/tmp CGO_ENABLED=1 go test -v -race -count 1 ${TESTPKGS}

# runs unit-tests
FROM base AS unit-tests-run
ARG TESTPKGS
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg --mount=type=cache,target=/tmp go test -v -covermode=atomic -coverprofile=coverage.txt -coverpkg=${TESTPKGS} -count 1 ${TESTPKGS}

FROM scratch AS talos-backup-linux-amd64
COPY --from=talos-backup-linux-amd64-build /talos-backup-linux-amd64 /talos-backup-linux-amd64

FROM scratch AS talos-backup-linux-arm64
COPY --from=talos-backup-linux-arm64-build /talos-backup-linux-arm64 /talos-backup-linux-arm64

FROM scratch AS unit-tests
COPY --from=unit-tests-run /src/coverage.txt /coverage.txt

FROM talos-backup-linux-${TARGETARCH} AS talos-backup

FROM scratch AS talos-backup-all
COPY --from=talos-backup-linux-amd64 / /
COPY --from=talos-backup-linux-arm64 / /

FROM scratch AS image-talos-backup
ARG TARGETARCH
COPY --from=talos-backup talos-backup-linux-${TARGETARCH} /talos-backup
COPY --from=image-fhs / /
COPY --from=image-ca-certificates / /
LABEL org.opencontainers.image.source https://github.com/siderolabs/talos-backup
ENTRYPOINT ["/talos-backup"]

