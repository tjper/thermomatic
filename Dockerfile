FROM golang:1.13 as builder

WORKDIR /app/thermomatic/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app/
COPY --from=builder /app/thermomatic/app .
CMD ["./app"]
