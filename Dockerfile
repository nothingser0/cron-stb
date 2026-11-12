FROM golang:alpine AS builder
WORKDIR /app

RUN apk add --no-cache ca-certificates git
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -buildvcs=false -ldflags="-s -w" -o /cron-stb ./cmd/cron-stb

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Jakarta

WORKDIR /app
COPY --from=builder /cron-stb /app/cron-stb

ENTRYPOINT ["/app/cron-stb"]
CMD ["daemon"]
