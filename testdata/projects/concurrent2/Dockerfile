FROM debian:stretch

COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

WORKDIR /data

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
