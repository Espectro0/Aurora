FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/aurora ./cmd

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

# marco.cornare.gov.co (SIATA/Cornare skills) serves only its leaf certificate
# and omits the Sectigo intermediate, so clients that don't chase AIA (Go's
# http.Client included) fail with "certificate signed by unknown authority".
# Install the missing intermediate so the chain resolves to a root we already trust.
COPY certs/sectigo-public-server-authentication-ca-dv-r36.pem /usr/local/share/ca-certificates/sectigo-public-server-authentication-ca-dv-r36.crt
RUN update-ca-certificates

WORKDIR /app
COPY --from=build /out/aurora ./aurora

ENTRYPOINT ["./aurora"]
