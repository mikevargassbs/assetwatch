FROM node:22-alpine AS frontend-build
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-build /web/dist ./web/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/seed-admin ./cmd/seed-admin

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/seed-admin /usr/local/bin/seed-admin
COPY db/migrations /app/db/migrations
WORKDIR /app
EXPOSE 4083
ENTRYPOINT ["/usr/local/bin/api"]
