FROM golang:1.25-alpine AS builder
WORKDIR /app
RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
COPY go.mod go.sum ./
RUN go mod download
COPY sqlc.yml ./
COPY internal/db/ ./internal/db
RUN sqlc generate
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main cmd/scribble.go

FROM gcr.io/distroless/base-debian13:nonroot AS final
WORKDIR /home/nonroot
COPY --from=builder /app/main ./main
USER nonroot:nonroot
EXPOSE 9000
ENV CONFIG_FILE=/config/config.yml
ENV TZ=UTC
ENTRYPOINT ["/home/nonroot/main"]