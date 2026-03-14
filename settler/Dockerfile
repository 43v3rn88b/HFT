FROM golang:1.26-alpine

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# We set CGO_ENABLED=0 to ensure the binary is statically linked
# This is a best practice for Alpine-based Go containers
RUN CGO_ENABLED=0 GOOS=linux go build -o settler .

CMD ["./settler"]