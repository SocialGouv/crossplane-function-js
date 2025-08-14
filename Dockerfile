ARG GO_VERSION=1.24
ARG NODE_VERSION=24-alpine

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

COPY --chown=1000:1000 yarn.lock .yarnrc.yml /app/
COPY --chown=1000:1000 .yarn /app/.yarn

# Instead of using corepack enable, we'll create a yarn wrapper script that uses the first file in .yarn/releases
RUN YARN_PATH=$(find /app/.yarn/releases -type f -name "yarn-*.cjs" | sort | head -1) && \
    echo '#!/bin/sh' > /usr/local/bin/yarn && \
    echo "exec $(which node) $YARN_PATH \"\$@\"" >> /usr/local/bin/yarn && \
    chmod +x /usr/local/bin/yarn && \
    chown -R 1000:1000 /app

USER 1000
WORKDIR /app

RUN yarn fetch workspaces focus @crossplane-js/server --production && yarn cache clean

COPY --chown=1000:1000 package.json tsconfig.json ./
COPY --chown=1000:1000 packages/ ./packages/

FROM alpine:3 AS certs
RUN apk --update add ca-certificates

FROM node:$NODE_VERSION

USER 1000
WORKDIR /app
ENTRYPOINT ["/app/xfuncjs-server"]

ENV NODE_OPTIONS="--experimental-strip-types --experimental-transform-types --no-warnings"
ENV NODE_NO_WARNINGS=1
ENV YARN_CACHE_FOLDER=/tmp/yarn-cache
ENV HOME=/tmp

COPY crossplane.yaml package.yaml /

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=go-builder --chown=1000:1000 /app/xfuncjs-server /app/xfuncjs-server

COPY --from=js-builder --chown=1000:1000 /app/node_modules /app/node_modules
COPY --from=js-builder --chown=1000:1000 /app/packages /app/packages
COPY --from=js-builder /app/package.json /app/tsconfig.json /app/.yarnrc.yml /app/
COPY --from=js-builder /app/.yarn /app/.yarn

# Create yarn alias for the Berry version after copying .yarn directory
USER root
RUN YARN_PATH=$(find /app/.yarn/releases -type f -name "yarn-*.cjs" | sort | head -1) && \
    echo '#!/bin/sh' > /usr/local/bin/yarn && \
    echo "exec $(which node) $YARN_PATH \"\$@\"" >> /usr/local/bin/yarn && \
    chmod +x /usr/local/bin/yarn
USER 1000
