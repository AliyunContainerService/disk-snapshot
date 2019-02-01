FROM centos:7.4.1708
LABEL maintainers="Alibaba Cloud Authors"
LABEL description="Alibaba Cloud CSI Plugin"

RUN yum install -y e4fsprogs

COPY nsenter /
COPY disk-snapshot /bin/disk-snapshot
RUN chmod +x /bin/disk-snapshot
RUN chmod 755 /nsenter

ENTRYPOINT ["/bin/disk-snapshot"]
