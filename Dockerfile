# syntax = docker/dockerfile-upstream:1.2.0-labs

# THIS FILE WAS AUTOMATICALLY GENERATED, PLEASE DO NOT EDIT.
#
# Generated on 2022-01-13T16:08:44Z by kres latest.

ARG TOOLCHAIN

# cleaned up specs and compiled versions
FROM scratch AS generate

FROM ghcr.io/talos-systems/ca-certificates:v0.3.0-12-g90722c3 AS image-ca-certificates

FROM ghcr.io/talos-systems/fhs:v0.3.0-12-g90722c3 AS image-fhs

# runs markdownlint
FROM node:14.8.0-alpine AS lint-markdown
RUN npm i -g markdownlint-cli@0.23.2
RUN npm i sentences-per-line@0.2.1
WORKDIR /src
COPY .markdownlint.json .
COPY ./README.md ./README.md
RUN markdownlint --ignore "CHANGELOG.md" --ignore "**/node_modules/**" --ignore '**/hack/chglog/**' --rules /node_modules/sentences-per-line/index.js .

# base toolchain image
FROM ${TOOLCHAIN} AS toolchain
RUN apk --update --no-cache add bash curl build-base protoc protobuf-dev

# build tools
FROM toolchain AS tools
ENV GO111MODULE on
ENV CGO_ENABLED 0
ENV GOPATH /go
RUN curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b /bin v1.42.1
ARG GOFUMPT_VERSION
RUN go install mvdan.cc/gofumpt/gofumports@${GOFUMPT_VERSION} \
	&& mv /go/bin/gofumports /bin/gofumports

# tools and sources
FROM tools AS base
WORKDIR /src
COPY ./go.mod .
COPY ./go.sum .
RUN --mount=type=cache,target=/go/pkg go mod download
RUN --mount=type=cache,target=/go/pkg go mod verify
COPY ./cmd ./cmd
COPY ./pkg ./pkg
RUN --mount=type=cache,target=/go/pkg go list -mod=readonly all >/dev/null

# runs gofumpt
FROM base AS lint-gofumpt
RUN find . -name '*.pb.go' | xargs -r rm
RUN find . -name '*.pb.gw.go' | xargs -r rm
RUN FILES="$(gofumports -l -local github.com/rsmitty/talos-backer-upper .)" && test -z "${FILES}" || (echo -e "Source code is not formatted with 'gofumports -w -local github.com/rsmitty/talos-backer-upper .':\n${FILES}"; exit 1)

# runs golangci-lint
FROM base AS lint-golangci-lint
COPY .golangci.yml .
ENV GOGC 50
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/root/.cache/golangci-lint --mount=type=cache,target=/go/pkg golangci-lint run --config .golangci.yml

# builds talos-backer-upper-linux-amd64
FROM base AS talos-backer-upper-linux-amd64-build
COPY --from=generate / /
WORKDIR /src/cmd/talos-backer-upper
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg GOARCH=amd64 GOOS=linux go build -ldflags "-s -w" -o /talos-backer-upper-linux-amd64

# builds talos-backer-upper-linux-arm64
FROM base AS talos-backer-upper-linux-arm64-build
COPY --from=generate / /
WORKDIR /src/cmd/talos-backer-upper
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg GOARCH=arm64 GOOS=linux go build -ldflags "-s -w" -o /talos-backer-upper-linux-arm64

# runs unit-tests with race detector
FROM base AS unit-tests-race
ARG TESTPKGS
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg --mount=type=cache,target=/tmp CGO_ENABLED=1 go test -v -race -count 1 ${TESTPKGS}

# runs unit-tests
FROM base AS unit-tests-run
ARG TESTPKGS
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg --mount=type=cache,target=/tmp go test -v -covermode=atomic -coverprofile=coverage.txt -coverpkg=${TESTPKGS} -count 1 ${TESTPKGS}

FROM scratch AS talos-backer-upper-linux-amd64
COPY --from=talos-backer-upper-linux-amd64-build /talos-backer-upper-linux-amd64 /talos-backer-upper-linux-amd64

FROM scratch AS talos-backer-upper-linux-arm64
COPY --from=talos-backer-upper-linux-arm64-build /talos-backer-upper-linux-arm64 /talos-backer-upper-linux-arm64

FROM scratch AS unit-tests
COPY --from=unit-tests-run /src/coverage.txt /coverage.txt

FROM talos-backer-upper-linux-${TARGETARCH} AS talos-backer-upper

FROM scratch AS image-talos-backer-upper
ARG TARGETARCH
COPY --from=talos-backer-upper talos-backer-upper-linux-${TARGETARCH} /talos-backer-upper
COPY --from=image-fhs / /
COPY --from=image-ca-certificates / /
LABEL org.opencontainers.image.source https://github.com/talos-systems/talos-backer-upper
ENTRYPOINT ["/talos-backer-upper"]

