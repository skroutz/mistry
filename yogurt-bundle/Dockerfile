FROM debian:stretch

COPY sources.list /etc/apt/sources.list
COPY bpo.list skroutz-stable.list skroutz-pu.list /etc/apt/sources.list.d/
COPY skroutz-archive-keyring.gpg /etc/apt/trusted.gpg.d/
COPY 01norecommend /etc/apt/apt.conf.d/

RUN apt-get update && apt-get install -y ssh ruby ruby-dev ruby-bundler
RUN apt-get install -y ruby2.3-dev build-essential \
                       libxml2-dev \
                       libxslt1-dev

RUN mkdir /data
WORKDIR /data

COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

#ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
