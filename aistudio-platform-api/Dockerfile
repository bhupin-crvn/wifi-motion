##v1.0.0.13

FROM golang:latest AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o main --buildvcs=false .

FROM golang:latest

WORKDIR /app

COPY --from=builder /app/main /app/main

EXPOSE 8080

CMD ["./main"]
