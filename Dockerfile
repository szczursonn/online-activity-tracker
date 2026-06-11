FROM golang:1.26.4 AS builder
WORKDIR /src

# cache deps
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /oat ./cmd/app

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /data
COPY --from=builder /oat /usr/local/bin/oat
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/oat"]
