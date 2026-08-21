# syntax=docker/dockerfile:1
#
# GoFitness is a pure-Go binary with an embedded frontend, so it cross-compiles
# for every Home Assistant architecture with CGO disabled. The builder runs
# natively on the build host ($BUILDPLATFORM) and emits a static binary for the
# target arch, which is then dropped onto the matching Home Assistant base image.

# BUILD_FROM must be a global ARG (declared before the first FROM) so it is in
# scope for the runtime stage's FROM line. The Home Assistant builder overrides
# it per architecture from build.yaml.
ARG BUILD_FROM=ghcr.io/home-assistant/amd64-base:3.21

# --- build stage ------------------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

# BUILD_ARCH is supplied by the Home Assistant builder (aarch64, amd64, ...).
ARG BUILD_ARCH=amd64
ARG BUILD_VERSION=dev

WORKDIR /src

# Cache modules separately so source-only changes don't re-download the graph.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Map the Home Assistant arch names onto GOARCH/GOARM.
RUN set -eux; \
    case "${BUILD_ARCH}" in \
      aarch64) export GOARCH=arm64 ;; \
      amd64)   export GOARCH=amd64 ;; \
      armhf)   export GOARCH=arm GOARM=6 ;; \
      armv7)   export GOARCH=arm GOARM=7 ;; \
      i386)    export GOARCH=386 ;; \
      *) echo "unsupported BUILD_ARCH=${BUILD_ARCH}" >&2; exit 1 ;; \
    esac; \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags "-s -w -X main.version=${BUILD_VERSION}" \
      -o /out/gofitness ./cmd/gofitness

# --- runtime stage ----------------------------------------------------------
FROM ${BUILD_FROM}

# Home Assistant add-on labels (also set by build.yaml at build time).
LABEL io.hass.type="addon"

COPY --from=builder /out/gofitness /usr/bin/gofitness
COPY run.sh /run.sh
RUN chmod a+x /run.sh /usr/bin/gofitness

# The database and options live under /data, which the Supervisor persists.
WORKDIR /data

CMD [ "/run.sh" ]
