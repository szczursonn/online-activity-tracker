# syntax=docker/dockerfile:1

FROM golang:1.26.5-alpine AS builder
ENV GOTOOLCHAIN=local
WORKDIR /src

# cache deps
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /oat .

FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=builder /oat /usr/local/bin/oat

WORKDIR /data
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/oat"]
