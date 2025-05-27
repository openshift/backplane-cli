# How to setup PS1 in bash/zsh

You can use the below methods to set the shell prompt, so you can easily see which cluster you are connected to, like:
~~~
[user@user ~ (⎈ |stg/user-test-1.zlob.s1:default)]$ date
Tue Sep  7 17:40:35 CST 2021
[user@user ~ (⎈ |stg/user-test-1.zlob.s1:default)]$ oc whoami
system:serviceaccount:default:xxxxxxxxxxxx
~~~

## Bash
Save the [kube-ps1](https://raw.githubusercontent.com/jonmosco/kube-ps1/master/kube-ps1.sh) script to local, and append the below to `~/.bashrc`.
~~~
source /path/to/kube-ps1.sh ##<---- replace this to the kube-ps1 location
function cluster_function() {
  info="$(ocm backplane status 2> /dev/null)"
  if [ $? -ne 0 ]; then return; fi
  clustername=$(grep "Cluster Name" <<< $info | awk '{print $3}')
  baseid=$(grep "Cluster Basedomain" <<< $info | awk '{print $3}' | cut -d'.' -f1,2)
  echo $clustername.$baseid
}
KUBE_PS1_BINARY=oc
export KUBE_PS1_CLUSTER_FUNCTION=cluster_function
PS1='[\u@\h \W $(kube_ps1)]\$ '
~~~

## Zsh

### With `oh-my-zsh` enabled
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
  info="$(ocm backplane status 2> /dev/null)"
  if [ $? -ne 0 ]; then return; fi
  clustername=$(grep "Cluster Name" <<< $info | awk '{print $3}')
  baseid=$(grep "Cluster Basedomain" <<< $info | awk '{print $3}' | cut -d'.' -f1,2)
  echo $clustername.$baseid
}
KUBE_PS1_BINARY=oc
export KUBE_PS1_CLUSTER_FUNCTION=cluster_function
PROMPT='$(kube_ps1)'$PROMPT
~~~
### Without `oh-my-zsh` enabled
Save the [kube-ps1](https://raw.githubusercontent.com/jonmosco/kube-ps1/master/kube-ps1.sh) script to local, and append the below to `~/.zshrc`.
~~~
source /path/to/kube-ps1.sh ##<---- replace this to your location
function cluster_function() {
  info="$(ocm backplane status 2> /dev/null)"
  if [ $? -ne 0 ]; then return; fi
  clustername=$(grep "Cluster Name" <<< $info | awk '{print $3}')
  baseid=$(grep "Cluster Basedomain" <<< $info | awk '{print $3}' | cut -d'.' -f1,2)
  echo $clustername.$baseid
}
KUBE_PS1_BINARY=oc
export KUBE_PS1_CLUSTER_FUNCTION=cluster_function
PS1='[\u@\h \W $(kube_ps1)]\$ '
~~~


## Disabling warning when PS1 is not configured

If you would like to disable warnings from `ocm-backplane` when `kube-ps1` is not configured, you can set the
`disable-kube-ps1-warning` value to `false` in your configuration file.
