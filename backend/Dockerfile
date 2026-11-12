FROM golang:1.27.3-alpine AS builder

WORKDIR /src

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /shorty .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates
COPY --from=builder /shorty /usr/local/bin/shorty

USER 65532:65532
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/shorty"]
