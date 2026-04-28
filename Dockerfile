FROM node:24-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend
RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/frontend/dist ./frontend/dist
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/ap-psych-final .

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend /out/ap-psych-final ./ap-psych-final
COPY data/sources ./data/sources
ENV APP_ENV=production \
    GIN_MODE=release \
    PORT=8080
EXPOSE 8080
CMD ["./ap-psych-final"]
