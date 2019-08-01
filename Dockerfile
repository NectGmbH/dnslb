FROM alpine:3.10

COPY ./dnslb /usr/local/bin/dnslb

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

ENTRYPOINT [ "/usr/local/bin/dnslb" ]