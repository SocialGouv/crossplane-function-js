FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
COPY vendor/ vendor/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -mod=vendor -o skyhook-server cmd/server/main.go

FROM node:22-alpine

# Install certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the Go binary
COPY --from=builder /app/skyhook-server /app/skyhook-server

# Copy Node.js files
COPY src/ /app/src/
COPY package.json yarn.lock .yarnrc.yml ./

# Install Node.js dependencies
RUN yarn install

EXPOSE 50051

ENTRYPOINT ["/app/skyhook-server"]
