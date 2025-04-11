# Use Go 1.24 (or higher) base image
FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o rbac-tool ./backend

#RUN go build -o rbac-tool ./backend

# Final image
FROM gcr.io/distroless/base-debian11

WORKDIR /app

COPY --from=builder /app/rbac-tool .

CMD ["./rbac-tool"]
