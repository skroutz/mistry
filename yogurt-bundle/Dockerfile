FROM debian:stretch

ARG ssh_key

COPY sources.list /etc/apt/sources.list
COPY bpo.list skroutz-stable.list skroutz-pu.list /etc/apt/sources.list.d/
COPY skroutz-archive-keyring.gpg /etc/apt/trusted.gpg.d/
COPY 01norecommend /etc/apt/apt.conf.d/

RUN apt-get update && apt-get install -y ssh ruby ruby-dev ruby-bundler

RUN useradd -ms /bin/bash wowbagger
USER wowbagger
WORKDIR /home/wowbagger

RUN mkdir bundle
WORKDIR /home/wowbagger/bundle

COPY --chown=wowbagger docker-entrypoint.sh /home/wowbagger
RUN chmod +x /home/wowbagger/docker-entrypoint.sh

ENTRYPOINT ["/home/wowbagger/docker-entrypoint.sh"]
