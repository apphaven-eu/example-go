FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app .

FROM alpine:3
RUN adduser -D -H -u 10001 app
COPY --from=build /out/app /usr/local/bin/app

USER app
ENV PORT=8080
EXPOSE 8080

CMD ["/usr/local/bin/app"]
