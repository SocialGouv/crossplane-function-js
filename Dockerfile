FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
COPY vendor/ vendor/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -mod=vendor -o skyhook-server cmd/server/main.go

FROM node:22-alpine

EXPOSE 9443

ENTRYPOINT ["/app/skyhook-server"]

# Install certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /app

RUN corepack enable

# Install JS dependencies
COPY yarn.lock .yarnrc.yml ./
COPY .yarn .yarn
RUN yarn fetch

# Copy TypeScript source files directly
COPY src/ ./src/
COPY package.json tsconfig.json ./

# Install Node.js dependencies (production only)
RUN yarn workspaces focus --production && yarn cache clean

# Set NODE_OPTIONS to enable running TypeScript directly
ENV NODE_OPTIONS="--no-warnings --experimental-strip-types "

# Copy crossplane files
COPY crossplane.yaml package.yaml /

# Copy the Go binary
COPY --from=builder /app/skyhook-server /app/skyhook-server

