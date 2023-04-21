#!/bin/bash -e

if [ "$I_AM_IN_CONTAINER" != "I-am-in-container" ]; then
  echo "must be run in container";
  exit 1;
fi

echo "in container";

mkdir /usr/local/backplane -p
pushd /usr/local/backplane

if [[ -d backplane-cli ]] ;
then
  rm -r ./backplane-cli
fi

git clone --single-branch --branch main https://github.com/openshift/backplane-cli.git
pushd backplane-cli
make build
mv ocm-backplane /usr/local/bin/ocm-backplane
chmod 755 /usr/local/bin/ocm-backplane

ocm-backplane completion bash > /etc/bash_completion.d/backplane-cli

popd
popd
