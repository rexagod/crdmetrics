FROM golang:latest as builder

WORKDIR /

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN make crdmetrics

FROM ubuntu:latest

RUN apt-get update && apt-get install -y ca-certificates

WORKDIR /

COPY --from=builder /crdmetrics .

CMD ["./crdmetrics"]
