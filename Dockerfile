# ---- build
FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/bot ./cmd/bot

# ---- run
FROM alpine:3.20
RUN adduser -D app
USER app
WORKDIR /home/app
COPY --from=build /out/bot /usr/local/bin/bot
ENTRYPOINT ["bot"]
