FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -buildvcs=false -ldflags="-s -w" -o /cron-stb ./cmd/cron-stb

FROM alpine:3.20

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Jakarta

COPY --from=builder /cron-stb /app/cron-stb

ENTRYPOINT ["/app/cron-stb"]
CMD ["daemon"]
