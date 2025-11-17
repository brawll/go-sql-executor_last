# Build stage
FROM golang:1.23 AS build
WORKDIR /app

# Optimization: Re-download dependencies only when
# go.mod or go.sum files change.
COPY go.* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o binary .

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /app/binary /app/binary

CMD ["/app/binary"]
