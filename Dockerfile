#
#   Author: Rohith (gambol99@gmail.com)
#   Date: 2015-03-24 10:51:35 +0000 (Tue, 24 Mar 2015)
#
#  vim:ts=2:sw=2:et
#
FROM progrium/busybox
MAINTAINER Rohith Jayawardene <gambol99@gmail.com>

ADD ./bin/bridge/bridge /bin/bridge
RUN opkg-install curl
RUN chmod +x /bin/bridge

EXPOSE 8080
ENTRYPOINT [ "/bin/bridge" ]
