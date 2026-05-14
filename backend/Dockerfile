FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o /bin/crowdrise-api ./cmd/api

FROM alpine:3.20
WORKDIR /app
COPY --from=build /bin/crowdrise-api /bin/crowdrise-api
COPY --from=build /app/internal/db/migrations /app/internal/db/migrations
EXPOSE 8080
CMD ["/bin/crowdrise-api"]
