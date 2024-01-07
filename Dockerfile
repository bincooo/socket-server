FROM golang:1.20-alpine AS builder

WORKDIR /app
RUN apk add git && git clone https://github.com/bincooo/socket-server.git .
RUN go mod tidy && GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o server -trimpath

FROM bincooo/chrome-vnc:latest
WORKDIR /app
COPY --from=builder /app/server ./server

ENV DISPLAY=:99
ENV APT_KEY_DONT_WARN_ON_DANGEROUS_USAGE=DontWarn
EXPOSE 8080

#ENTRYPOINT ["tail","-f","/dev/null"]
ENTRYPOINT ["/bin/bash", "/app/docker-entrypoint.sh"]