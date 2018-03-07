FROM debian:stretch

ARG ssh_key
ARG uid

COPY sources.list /etc/apt/sources.list
COPY bpo.list skroutz-stable.list skroutz-pu.list /etc/apt/sources.list.d/
COPY skroutz-archive-keyring.gpg /etc/apt/trusted.gpg.d/
COPY 01norecommend /etc/apt/apt.conf.d/

RUN apt-get update && apt-get install -y ssh git ruby ruby-dev ruby-bundler
RUN apt-get install -y build-essential \
                       libcurl4-openssl-dev \
                       libmariadbclient-dev \
                       mariadb-client \
                       libxml2-dev \
                       libxslt1-dev \
                       libsystemd-dev \
                       imagemagick \
                       gettext \
                       libicu-dev \
                       cmake \
                       pkg-config \
                       libmagic-dev \
                       libsqlite3-dev \
                       nodejs \
                       nodejs-legacy \
								vim \
                       yarn


RUN useradd -ms /bin/bash -u "$uid" builder
USER builder
WORKDIR /home/builder

RUN mkdir .ssh
RUN echo "$ssh_key" > .ssh/id_rsa
RUN chmod 700 .ssh/id_rsa
RUN echo "Host github.skroutz.gr\n\tStrictHostKeyChecking no\n" > .ssh/config

# should probably be in cache/yogurt
RUN git clone git@github.skroutz.gr:skroutz/yogurt.git
WORKDIR yogurt
RUN bundle --deployment --without=development test
RUN yarn install

USER root
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

USER builder
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
