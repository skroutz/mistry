FROM debian:stretch

RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    libcurl4-openssl-dev \
    libsystemd-dev \
    pkg-config \
    ruby ruby-dev ruby-bundler \
    zlib1g-dev

WORKDIR /data

COPY docker-entrypoint /usr/local/bin/docker-entrypoint
RUN chmod +x /usr/local/bin/docker-entrypoint

ENTRYPOINT ["/usr/local/bin/docker-entrypoint"]
