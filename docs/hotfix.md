# How to make a hotfix release

This document describes how to generate a hotfix release for backplane. 

### When to make a hotfix release

- A critical bug has been found
- Security vulnerabilities have been encountered
- Something broke in the last/latest release

### Hotfix release process technical guidelines

Please make sure to adhere to these following guidelines below for the hotfix release process : 

#### 1. Create an OSD card

- Create an OSD card with title : **Backplane Hotfix Release for <$reason>**
- In the description of the card , mention details about the intention/reason behind this hotfix.
- Add backplane-hotfix label to it.

#### 2. Create pull request 

The main aim is to put hotfixes to production without promoting all changes from the main branch.

- Create a branch called **hotfix** in your code repository. 
NOTE: This branch should be created from the commit that is currently deployed to production.

- ##### If the hotfix is on reverting a commit : 
    1. Use ```git log``` to get the commit hash of the targeted commit.
    1. Once you get the commit hash , use git revert with -m flag.
       In git revert -m, the -m option specifies the parent number. When you view a merge commit in the output of git log, you will see its parents listed on the line that begins with **Merge:**
    1. Execute ```git revert -m <$parent number> <$commit-hash> ```
    1. ```git push -u origin {hotfix-branch}```

- ##### If there are code changes to be made : 
    1. Make applicable code changes.
    1. Raise PR for the same following the usual PR process

Once you raise the PR, mention the OSD card created in step1 in the description of the PR.

#### 3. Reach out to the backplane-team

Once you create the PR : 

1. Reach out to **@backplane-team** in [#sd-ims-backplane](https://app.slack.com/client/E030G10V24F/C016S65RNG5) slack channel asking for review and merge.
1. Once the PR is merged: 
    - In case of backplane-cli, ask **@Bo Meng** to cut a new backplane release.
    - In case of backplane-api, create a promotion MR in app-interface by following :  
    ```osdctl promote saas -- serviceName saas-backplane-api --gitHash <use commit sha from the hotfix branch that you created>```
    - Reach out to **@saas-osd-operators** in [#sre-operators](https://app.slack.com/client/E030G10V24F/CFJD1NZFT) channel asking to merge

#### 4. How to announce hotfix release

Send an email to **backplane-announce@redhat.com** . Include details on the purpose behind the release.

You can also use the below format template (not compulsory): 

```bash
To : backplane-announce@redhat.com
Subject : Backplane hotfix release for <reason>

Message : 
Greetings everyone!
This email is in context of the recent backplane hotfix release that can also be referenced via <OSD Card created>.
With this fix in place, we will no longer encounter <reasons behind hotfix and its impact> 
If you have any questions, please feel free to reach out to me or backplane-team in #sd-ims-backplane slack channel.

Thanks
```

