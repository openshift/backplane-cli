### Download the binaries
# Anything in this image must be COPY'd into the final image, below
FROM registry.access.redhat.com/ubi8/ubi as builder

RUN dnf update -y && dnf install -y git golang make && dnf clean all

ENV I_AM_IN_CONTAINER="I-am-in-container"

# Add RH-IT-Root-CA to enable secure internal repo cloning
ADD https://password.corp.redhat.com/RH-IT-Root-CA.crt /etc/pki/ca-trust/source/anchors/
RUN update-ca-trust

ADD ./container-setup/install /container-setup/install
WORKDIR /container-setup/install

RUN ./install-backplane.sh

FROM ocm-container:latest

RUN microdnf --assumeyes install \
    podman \
    fuse-overlayfs \
    shadow-utils \
    crun

# Overlay over overlay is often denied by the kernel, so this creates non overlay volumes to be used within the container.
VOLUME /var/lib/containers

# adjust storage.conf to enable fuse-overlayfs storage.
RUN sed -i -e 's|^#mount_program|mount_program|g' -e 's|^mountopt[[:space:]]*=.*$|mountopt = "nodev,fsync=0"|g' /etc/containers/storage.conf

# add containers.conf file to make sure containers run easier.
COPY containers.conf /etc/containers/containers.conf

ENV I_AM_IN_CONTAINER="I-am-in-container"

RUN mkdir -p /root/.config/backplane
COPY backplane.config.temp /root/.config/backplane/config.json

COPY --from=builder /usr/local/bin/ocm-backplane /usr/local/bin
COPY --from=builder /etc/bash_completion.d/backplane-cli /etc/bash_completion.d

ADD ./container-setup/utils /container-setup/utils
WORKDIR /container-setup/utils
RUN ./install-utils.sh

ENV PATH "$PATH:/root/utils"
RUN rm -rf /container-setup

WORKDIR /root
ENTRYPOINT ["/bin/bash"]
