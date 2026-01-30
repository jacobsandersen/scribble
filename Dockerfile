FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main cmd/scribble.go

FROM gcr.io/distroless/base-debian13:nonroot AS final
WORKDIR /root
RUN mkdir -p /config
COPY --from=builder /app/main .
EXPOSE 9000
ENV CONFIG_FILE=/config/config.yml
CMD ["./main"]