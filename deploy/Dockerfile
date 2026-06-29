FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/scenarios ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /out/scenarios /usr/local/bin/scenarios
EXPOSE 19095 9099
ENTRYPOINT ["scenarios"]
