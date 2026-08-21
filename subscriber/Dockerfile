FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
WORKDIR /src/subscriber
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/radius-subscriber ./

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/radius-subscriber /usr/local/bin/radius-subscriber
ENV REDIS_ADDR=redis:6379
ENV LOG_FILE=/var/log/radius_updates.log
CMD ["/usr/local/bin/radius-subscriber"]
