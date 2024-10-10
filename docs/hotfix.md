# How to make a hotfix release

This document describes how to generate a hotfix release for backplane-cli. 

### When to make a hotfix release

- A critical bug has been found
- Security vulnerabilities have been encountered
- Something broke in the last/latest release

### Hotfix release process technical guidelines

Please make sure to adhere to these following guidelines below for the hotfix release process : 

#### 1. Create an OSD card

- Create an OSD card with title : **Backplane Hotfix Release for <$reason>**
- In the description of the card , mention details about the intention/reason behind this hotfix.
- Set the Priority of this card as High.
- Add backplane-hotfix label to it.

#### 2. Create pull request 

- Create a branch called **hotfix-$OSD-card-number** in your code repository. 

**NOTE** : We strongly recommend that all the changes are well tested and validated before commiting them.

- ##### If the hotfix is on reverting a commit : 
    1. Use ```git log``` to get the commit hash of the targeted commit.
    1. Once you get the commit hash , use git revert with -m flag.
       In git revert -m, the -m option specifies the parent number. When you view a merge commit in the output of git log, you will see its parents listed on the line that begins with **Merge:**
    1. Execute ```git revert -m <$parent number> <$commit-hash> ```
    1. ```git push -u origin {hotfix-branch}```

    Alternatively, you can get backplane maintainers to use GitHub's UI to revert a commit directly from the merged PR, which simplifies the process and avoids potential errors in command execution.

- ##### If there are code changes to be made : 
    1. Make applicable code changes.
    1. Raise PR for the same following the usual PR process

Once you raise the PR, mention the OSD card created in step1 in the description of the PR.

#### 3. Reach out to the backplane-team

Once you create the PR : 

1. Reach out to **@backplane-team** in #sd-ims-backplane slack channel asking for review and merge.
1. Once the PR is merged, ask **@Backplane-cli** to cut a new backplane release.

#### 4. How to announce hotfix release

Send an email to applicable stakeholders . Include details on the purpose behind the release.
