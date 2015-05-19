FROM 192.168.5.46:5000/armhf-ubuntu:14.04

MAINTAINER caizi Deepglint <chengzhang@deepglint.com>

RUN mkdir /webhook

ADD webhook.arm /usr/bin/

ADD hooks.json shell/  /webhook/

EXPOSE 8010

WORKDIR /webhook

CMD webhook.arm -hooks="hooks.json" -hotreload=true -port=8010 -urlprefix="hooks" -verbose=true