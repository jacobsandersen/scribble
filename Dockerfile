FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main cmd/scribble.go

FROM gcr.io/distroless/base-debian13:nonroot AS final
WORKDIR /home/nonroot
COPY --from=builder /app/main ./main
USER nonroot:nonroot
EXPOSE 9000
ENV CONFIG_FILE=/config/config.yml
ENTRYPOINT ["/home/nonroot/main"]