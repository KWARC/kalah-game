FROM docker.io/library/alpine:3.14 as os

# install ca-certificates
RUN apk add --update --no-cache ca-certificates

# create www-data
RUN set -x ; \
  addgroup -g 82 -S www-data ; \
  adduser -u 82 -D -S -G www-data www-data && exit 0 ; exit 1

# build the backend
FROM docker.io/library/golang:1.17 as builder
ADD . /app/
WORKDIR /app/
RUN go get .
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-linkmode external -extldflags "-static"' -o go-kgp ./

# add it into a new image
FROM docker.io/library/alpine:3.14

# add the user
COPY --from=os /etc/passwd /etc/passwd
COPY --from=os /etc/group /etc/group

# create a volume at /data/
RUN mkdir /data/ && chown -R www-data:www-data /data/
VOLUME /data/

# add the app
COPY --from=builder /app/go-kgp /go-kgp

# and set the entry command
EXPOSE 8080
EXPOSE 2671
USER www-data:www-data
CMD ["/go-kgp", "-db", "/data/kalah.sql", "-webhost", "0.0.0.0", "-webport", "8080", "-host", "0.0.0.0", "-port", "2671"]