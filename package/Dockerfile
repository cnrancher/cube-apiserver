FROM ubuntu:16.04
COPY rancher-cube-apiserver /usr/bin/
RUN mkdir -p /etc/rancher/cube
COPY id_rsa /etc/rancher/cube
COPY id_rsa.pub /etc/rancher/cube
RUN chmod 600 /etc/rancher/cube/id_rsa*
ENTRYPOINT ["rancher-cube-apiserver"]
