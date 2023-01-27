#!/bin/bash -e

if [ "$I_AM_IN_CONTAINER" != "I-am-in-container" ]; then
  echo "must be run in container";
  exit 1;
fi

echo "in container";

mkdir -p ${HOME}/utils

mv /container-setup/utils/bin/* ${HOME}/utils

# find *-sre-utils.bashrc in ~/.bashrc.d
BASHRC_DIR=${HOME}/.bashrc.d
FIND_PATTERN="*-sre-utils.bashrc"
SRE_UTILS_BASHRC=$(find ${BASHRC_DIR} -iname ${FIND_PATTERN} -print -quit)
if [ -z ${SRE_UTILS_BASHRC} ]
then
  echo "Expecting to find '${FIND_PATTERN}' file in ${BASHRC_DIR}"
  exit 1
fi

# change *-login to backplane-login in bashrc
sed -i 's/\w*-login/backplane-login/g' ${SRE_UTILS_BASHRC}

# change cluster_function for PS1
# add a new line ahead as sre-utils.bashrc may not end with a new line
cat >> ${SRE_UTILS_BASHRC} << EOF

function cluster_function() {
  info="\$(ocm backplane status)"
  clustername=\$(echo "\$info" | grep "Cluster Name" | awk '{print \$3}')
  baseid=\$(echo "\$info" | grep "Cluster Basedomain" | awk '{print \$3}' | cut -d'.' -f1,2)
  echo \$clustername.\$baseid
}
EOF

# Add elevation alias
cat >> ${SRE_UTILS_BASHRC} << EOF

alias oc-elevate="oc --as backplane-cluster-admin"
EOF
