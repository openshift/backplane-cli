# How to setup PS1 in bash/zsh

You can use the below methods to set the shell prompt, so you can learn which cluster operating on, like
~~~
[user@user ~ (⎈ |stg/user-test-1.zlob.s1:default)]$ date
Tue Sep  7 17:40:35 CST 2021
[user@user ~ (⎈ |stg/user-test-1.zlob.s1:default)]$ oc whoami
system:serviceaccount:openshift-backplane-srep:xxxxxxxxxxxx
~~~

## Bash
Save the [kube-ps1](https://raw.githubusercontent.com/jonmosco/kube-ps1/master/kube-ps1.sh) script to local, and append the below to `~/.bashrc`.
~~~
source /path/to/kube-ps1.sh ##<---- replace this to the kube-ps1 location
function cluster_function() {
  ocm_config="$(oc config view --minify -o jsonpath='{.users[0].user.exec.env[?(@.name == "OCM_CONFIG")].value}' 2> /dev/null)"
  info="$(OCM_CONFIG=$ocm_config ocm backplane status 2> /dev/null)"
  if [ $? -ne 0 ]; then return; fi
  clustername=$(grep "Cluster Name" <<< $info | awk '{print $3}')
  baseid=$(grep "Cluster Basedomain" <<< $info | awk '{print $3}' | cut -d'.' -f1,2)
  echo $clustername.$baseid
}
KUBE_PS1_BINARY=oc
KUBE_PS1_CLUSTER_FUNCTION=cluster_function
PS1='[\u@\h \W $(kube_ps1)]\$ '
~~~

## Zsh

kube-ps1 is included as a plugin in the oh-my-zsh project. To enable it, edit your `~/.zshrc` and add the plugin:

```
plugins=(
  kube-ps1
)
```

Save the [kube-ps1](https://raw.githubusercontent.com/jonmosco/kube-ps1/master/kube-ps1.sh) script to local, and append the below to `~/.zshrc`.
~~~
source /path/to/kube-ps1.sh ##<---- replace this to your location
function cluster_function() {
  ocm_config="$(oc config view --minify -o jsonpath='{.users[0].user.exec.env[?(@.name == "OCM_CONFIG")].value}' 2> /dev/null)"
  info="$(OCM_CONFIG=$ocm_config ocm backplane status 2> /dev/null)"
  if [ $? -ne 0 ]; then return; fi
  clustername=$(grep "Cluster Name" <<< $info | awk '{print $3}')
  baseid=$(grep "Cluster Basedomain" <<< $info | awk '{print $3}' | cut -d'.' -f1,2)
  echo $clustername.$baseid
}
KUBE_PS1_BINARY=oc
KUBE_PS1_CLUSTER_FUNCTION=cluster_function
PROMPT='$(kube_ps1)'$PROMPT
~~~
