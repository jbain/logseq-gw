# syntax=docker/dockerfile:1

# logseq-gw + the logseq CLI from a Logseq nightly build.
#
# Targets linux/arm64 (Rockchip SBCs, Apple Silicon under Docker Desktop).
#
# Logseq publishes no standalone CLI: it ships inside the desktop Electron
# bundle as resources/app.asar/js/logseq-cli.js. Electron itself is not
# needed to run it -- the CLI and db-worker-node are plain Node programs and
# their one native dependency (keytar) is N-API, so they run on the stock
# Node runtime once the asar is unpacked. The image therefore keeps the app
# bundle's JS and drops the ~180MB Electron runtime with it.
#
# NODE_TAG must stay on the Node major that the bundle's Electron embeds
# (Electron 42 -> Node 24); bump it when a nightly moves to a newer Electron.

ARG NODE_TAG=24-slim

# ---------------------------------------------------------------- gateway ---
FROM --platform=$BUILDPLATFORM golang:1.26-trixie AS gw

WORKDIR /src
COPY go.mod ./
COPY main.go ./
COPY internal ./internal
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/logseq-gw .

# ------------------------------------------------------------ logseq fetch ---
# Runs on the build host: this stage only downloads and unpacks JS, and picks
# the asset by TARGETARCH, so keeping it native saves emulating the unpack on
# cross-arch builds.
FROM --platform=$BUILDPLATFORM node:${NODE_TAG} AS logseq

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl unzip \
 && rm -rf /var/lib/apt/lists/*

# Which release to pull the desktop bundle from. `nightly` is the rolling
# nightly tag; pin it to e.g. `2.0.1` for a reproducible build.
ARG LOGSEQ_RELEASE=nightly
ARG TARGETARCH

# `nightly` is a rolling tag, so the download layer below would otherwise be
# cached against a stale build forever. Change this value (CI passes the build
# date) to force a re-resolve.
ARG LOGSEQ_REFRESH=0

# Resolve the linux zip asset for this architecture out of the release,
# unpack it, then extract the asar into /opt/logseq. The zip's
# app.asar.unpacked sits beside app.asar and holds the native modules the
# asar only references, so it has to be in place before extraction.
RUN set -eu; \
    case "${TARGETARCH}" in \
      arm64) pattern='Logseq-linux-arm64-.*\.zip$' ;; \
      amd64) pattern='Logseq-linux-x86_64-.*\.zip$' ;; \
      *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    url="$(curl -fsSL "https://api.github.com/repos/logseq/logseq/releases/tags/${LOGSEQ_RELEASE}" \
      | grep -o '"browser_download_url": *"[^"]*"' \
      | cut -d'"' -f4 \
      | grep -E "${pattern}" \
      | head -n1)"; \
    test -n "${url}"; \
    echo "downloading ${url}"; \
    curl -fsSL -o /tmp/logseq.zip "${url}"; \
    unzip -q /tmp/logseq.zip 'resources/*' -d /tmp/app; \
    npx --yes @electron/asar extract /tmp/app/resources/app.asar /opt/logseq; \
    rm -rf /tmp/logseq.zip /tmp/app; \
    find /opt/logseq -name '*.map' -delete; \
    test -f /opt/logseq/js/logseq-cli.js; \
    test -f /opt/logseq/js/db-worker-node.js

# ---------------------------------------------------------------- runtime ---
FROM node:${NODE_TAG}

# keytar, the CLI's only native dependency, dlopens libsecret at startup.
RUN apt-get update \
 && apt-get install -y --no-install-recommends libsecret-1-0 tini \
 && rm -rf /var/lib/apt/lists/*

COPY --from=logseq /opt/logseq /opt/logseq
COPY --from=gw /out/logseq-gw /usr/local/bin/logseq-gw

RUN printf '%s\n' \
      '#!/bin/sh' \
      '# logseq CLI from the unpacked app bundle (see Dockerfile)' \
      'exec node /opt/logseq/js/logseq-cli.js "$@"' \
    > /usr/local/bin/logseq \
 && chmod +x /usr/local/bin/logseq

# Graph files and logseq's own config live under one writable root.
ENV GATEWAY_PORT=8085 \
    GATEWAY_ROOT_DIR=/data/logseq \
    GATEWAY_LOGSEQ_BIN=logseq \
    HOME=/data \
    XDG_CONFIG_HOME=/data/.config \
    XDG_CACHE_HOME=/data/.cache

# uid 1000 is taken by the `node` user in these images; reuse it.
RUN mkdir -p /data/logseq && chown -R 1000:1000 /data

USER 1000:1000
WORKDIR /data
VOLUME ["/data"]
EXPOSE 8085

# tini reaps the db-worker-node children the gateway spawns.
ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["logseq-gw"]
