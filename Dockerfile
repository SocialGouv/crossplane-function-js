ARG GO_VERSION=1.23
ARG NODE_VERSION=22-alpine

FROM golang:$GO_VERSION AS go-builder

WORKDIR /app

COPY go.mod go.sum ./
COPY vendor/ vendor/
# COPY . .
COPY go.mod go.sum ./
COPY pkg/ pkg/
COPY cmd/ cmd/
COPY proto/ proto/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -mod=vendor -o skyhook-server cmd/server/main.go

FROM node:$NODE_VERSION AS js-builder

RUN apk add --no-cache jq

RUN corepack enable

USER 1000
WORKDIR /app

COPY --chown=1000:1000 yarn.lock .yarnrc.yml ./
COPY --chown=1000:1000 .yarn .yarn
RUN yarn fetch

COPY --chown=1000:1000 package.json tsconfig.json sea-config.json ./
COPY --chown=1000:1000 src/ ./src/
COPY --chown=1000:1000 packages/ ./packages/
RUN yarn build

RUN yarn workspaces focus crossplane-skyhook skyhook-sdk --production && yarn cache clean

# Collect platform-specific dependencies # see also https://dev.to/zavoloklom/how-to-build-multi-platform-executable-binaries-in-nodejs-with-sea-rollup-docker-and-github-d0g
USER root
SHELL ["/bin/ash", "-o", "pipefail", "-c"]
RUN mkdir -p /dependencies/lib /dependencies/usr/lib && \
  ldd /app/build/crossplane-skyhook | awk '{print $3}' | grep -vE '^$' | while read -r lib; do \
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
ENTRYPOINT ["/app/skyhook-server"]

# ENV NODE_OPTIONS="--no-warnings --experimental-strip-types "
ENV NODE_OPTIONS="--experimental-loader ./node_modules/node-ts-modules/ts-module-loader.mjs --no-warnings --experimental-strip-types "
ENV NODE_NO_WARNINGS=1
ENV YARN_CACHE_FOLDER=/tmp/yarn-cache
ENV HOME=/tmp

COPY crossplane.yaml package.yaml /

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=go-builder --chown=1000:1000 /app/skyhook-server /app/skyhook-server

COPY --from=js-builder --chown=1000:1000 /app/node_modules /app/node_modules
COPY --from=js-builder --chown=1000:1000 /app/packages /app/packages
COPY --from=js-builder /app/package.json /app/tsconfig.json /app/build/crossplane-skyhook /app/.yarnrc.yml /app/
COPY --from=js-builder /app/.yarn /app/.yarn

COPY --from=js-builder /dependencies/lib /lib
COPY --from=js-builder /dependencies/usr/lib /usr/lib
