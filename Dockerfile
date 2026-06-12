# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" -o /remarker ./cmd/main.go

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates

RUN addgroup -S remarker && adduser -S remarker -G remarker

WORKDIR /home/remarker

RUN mkdir -p documents

USER remarker

COPY --from=builder /remarker /usr/local/bin/remarker

VOLUME ["/home/remarker/documents"]

ENV REMARKER_SYNC_DIR=/home/remarker/documents

ENTRYPOINT ["remarker"]
CMD ["--help"]
