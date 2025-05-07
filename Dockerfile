ARG GO_VERSION=1.24
ARG NODE_VERSION=22-alpine

FROM golang:$GO_VERSION AS go-builder

WORKDIR /app

COPY go.mod go.sum ./
COPY vendor/ vendor/
# COPY . .
COPY go.mod go.sum ./
COPY pkg/ pkg/
COPY cmd/ cmd/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -mod=vendor -o xfuncjs-server cmd/server/main.go

FROM node:$NODE_VERSION AS js-builder

RUN apk add --no-cache jq


COPY --chown=1000:1000 yarn.lock .yarnrc.yml /app/
COPY --chown=1000:1000 .yarn /app/.yarn

# RUN corepack enable
# Instead of using corepack enable, we'll create a yarn wrapper script that uses the first file in .yarn/releases
RUN YARN_PATH=$(find /app/.yarn/releases -type f -name "yarn-*.cjs" | sort | head -1) && \
    echo '#!/bin/sh' > /usr/local/bin/yarn && \
    echo "exec $(which node) $YARN_PATH \"\$@\"" >> /usr/local/bin/yarn && \
    chmod +x /usr/local/bin/yarn && \
    chown -R 1000:1000 /app

USER 1000
WORKDIR /app

RUN yarn fetch

COPY --chown=1000:1000 package.json tsconfig.json ./
COPY --chown=1000:1000 packages/server/sea-config.json ./packages/server/
COPY --chown=1000:1000 packages/ ./packages/
RUN yarn build

RUN yarn workspaces focus @crossplane-js/server @crossplane-js/sdk --production && yarn cache clean
# RUN yarn fetch-tools production focus @crossplane-js/server @crossplane-js/sdk --production && yarn cache clean

# Collect platform-specific dependencies # see also https://dev.to/zavoloklom/how-to-build-multi-platform-executable-binaries-in-nodejs-with-sea-rollup-docker-and-github-d0g
USER root
SHELL ["/bin/ash", "-o", "pipefail", "-c"]
RUN mkdir -p /dependencies/lib /dependencies/usr/lib && \
    ldd /app/packages/server/build/xfuncjs-server-js | awk '{print $3}' | grep -vE '^$' | while read -r lib; do \
    if [ -f "$lib" ]; then \
    if [ "${lib#/usr/lib/}" != "$lib" ]; then \
    cp "$lib" /dependencies/usr/lib/; \
    elif [ "${lib#/lib/}" != "$lib" ]; then \
    cp "$lib" /dependencies/lib/; \
    fi; \
    fi; \
    done


FROM alpine:3 AS certs
RUN apk --update add ca-certificates

# FROM node:$NODE_VERSION
# FROM gcr.io/distroless/nodejs22-debian12
# FROM gcr.io/distroless/base
# FROM busybox
FROM scratch

USER 1000
WORKDIR /app
ENTRYPOINT ["/app/xfuncjs-server"]

# ENV NODE_OPTIONS="--no-warnings --experimental-strip-types "
ENV NODE_OPTIONS="--experimental-loader ./node_modules/node-ts-modules/loader-esm.mjs --no-warnings --experimental-strip-types "
ENV NODE_NO_WARNINGS=1
ENV YARN_CACHE_FOLDER=/tmp/yarn-cache
ENV HOME=/tmp

COPY crossplane.yaml package.yaml /

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=go-builder --chown=1000:1000 /app/xfuncjs-server /app/xfuncjs-server

COPY --from=js-builder --chown=1000:1000 /app/node_modules /app/node_modules
COPY --from=js-builder --chown=1000:1000 /app/packages /app/packages
COPY --from=js-builder /app/packages/server/build/xfuncjs-server-js /app/
COPY --from=js-builder /app/package.json /app/tsconfig.json /app/.yarnrc.yml /app/
COPY --from=js-builder /app/.yarn /app/.yarn

COPY --from=js-builder /dependencies/lib /lib
COPY --from=js-builder /dependencies/usr/lib /usr/lib
