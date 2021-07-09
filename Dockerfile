FROM scratch
COPY booster* /booster

EXPOSE 5000/tcp

# HACK: scratch does not have mkdir, and Golang requires a temporary directory
# see https://github.com/golang/go/issues/14196
COPY tmp /tmp

VOLUME ["/var/lib/registry"]

ENTRYPOINT ["/booster", "serve", "/var/lib/registry"]
