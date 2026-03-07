FROM ubuntu:latest
LABEL authors="lhecker"

ENTRYPOINT ["top", "-b"]

# todo: add postgresql, caddy, api, web