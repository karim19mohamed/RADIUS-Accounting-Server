FROM golang:1.22-alpine AS builder
WORKDIR /src
# copy module files from the repository root into build context
COPY go.mod go.sum ./
RUN go mod download
# copy entire repo into the container and build the server
COPY . /src
WORKDIR /src/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/radius-server ./

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/radius-server /usr/local/bin/radius-server
EXPOSE 1813/udp
ENV RADIUS_SECRET=testing123
ENV REDIS_ADDR=redis:6379
CMD ["/usr/local/bin/radius-server"]
