FROM docker.io/library/golang:alpine

# Fetch build dependencies
RUN apk add --no-cache sqlite graphviz gcc libc-dev

# Copy source into container and build the program
ADD . /app/
WORKDIR /app/
RUN go mod download
ARG CGO_CFLAGS="-D_LARGEFILE64_SOURCE"
RUN go build -v -o /go-kgp ./cmd/practice

# Mount an external volume and use it as a working directory
VOLUME /data/

# Expose end-points and start the server
EXPOSE 8080
EXPOSE 2671
ENTRYPOINT ["/go-kgp"]
