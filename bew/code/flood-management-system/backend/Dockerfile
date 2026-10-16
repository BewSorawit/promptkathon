FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /flood-api ./cmd/api
RUN CGO_ENABLED=0 go build -o /import-csv ./cmd/import-csv

FROM alpine:3.22
RUN adduser -D app
USER app
COPY --from=build /flood-api /flood-api
COPY --from=build /import-csv /import-csv
EXPOSE 8080
ENTRYPOINT ["/flood-api"]

