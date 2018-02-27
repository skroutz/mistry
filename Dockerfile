ARG base=debian:stretch
FROM $base

ARG foo=bar
RUN echo && echo $foo && echo && cat /etc/os-release | grep VERSION=
