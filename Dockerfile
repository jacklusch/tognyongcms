FROM node:20-alpine AS spa
WORKDIR /app/admin
ENV NODE_OPTIONS=--max-old-space-size=768
COPY admin/package.json admin/package-lock.json ./
RUN npm ci
COPY admin/ ./
RUN npm run build

FROM golang:1.22-alpine AS build
WORKDIR /src
ENV GOMEMLIMIT=700MiB
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=spa /app/admin/dist ./internal/server/dist
RUN CGO_ENABLED=0 go build -o /dulizhan ./cmd/dulizhan

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache tzdata ca-certificates && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && echo Asia/Shanghai > /etc/timezone
COPY --from=build /dulizhan .
COPY themes ./themes
EXPOSE 8080
ENTRYPOINT ["./dulizhan"]
