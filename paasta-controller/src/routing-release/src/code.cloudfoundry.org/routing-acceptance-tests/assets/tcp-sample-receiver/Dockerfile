FROM busybox:ubuntu-14.04
RUN mkdir -p /run
ADD https://s3.amazonaws.com/router-release-blobs/tcp-sample-receiver.linux /run/tcp-sample-receiver
RUN chmod 755 /run/tcp-sample-receiver
EXPOSE 5222
CMD /run/tcp-sample-receiver -address 0.0.0.0:5222 -serverId docker-server1
