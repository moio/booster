FROM scratch
COPY booster* /booster

EXPOSE 5000/tcp

VOLUME ["/var/lib/registry"]

ENTRYPOINT ["/booster", "serve"]
