FROM centos:7

RUN yum -y update
RUN yum install gcc git libmpfr4 -y && yum clean all
ENV GO_VERSION 1.20.9
RUN curl -L https://golang.org/dl/go$GO_VERSION.linux-amd64.tar.gz -o go$GO_VERSION.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go$GO_VERSION.linux-amd64.tar.gz && rm -f go$GO_VERSION.linux-amd64.tar.gz
ENV PATH /usr/local/go/bin:$PATH
ENV GOROOT /usr/local/go

# install slurm deps
RUN yum -y install \
  boost-devel \
  glib2-devel \
  glibc-devel \
  glibc-headers \
  glibc.i686 \
  gperftools \
  gperftools-devel \
  json-c-devel \
  libcmocka-devel \
  libedit \
  numactl-libs \
  openssh-clients \
  redhat-lsb-core \
  slurm \
  which \
  dtc \
  rsync && yum clean all