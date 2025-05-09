FROM golang:1.24.3-alpine3.21 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /kotei ./main.go

FROM alpine:3.21.3 AS final

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /kotei .

RUN addgroup -S kotei && adduser -S -G kotei kotei

USER kotei

ENV TZ=Etc/UTC

ENTRYPOINT ["./kotei"]
