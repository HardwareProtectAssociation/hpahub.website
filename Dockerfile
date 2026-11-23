FROM node:22-alpine AS frontend
WORKDIR /app
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /pocketbase .

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend /pocketbase /app/pocketbase
COPY --from=frontend /app/dist /app/pb_public
EXPOSE 8090
VOLUME ["/app/pb_data"]
CMD ["./pocketbase", "serve", "--http=0.0.0.0:8090"]
