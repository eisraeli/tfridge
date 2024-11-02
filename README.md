# tfridge - Keep your Terraform dependencies fresh !

![tfridge logo](img/logo.png)


# Introduction

tfridge aims at:

1. Listing the versions of all of your Terraform modules.
2. Listing the versions of all of your Terraform providers.
3. Lightweight tool that is easy to run locally or as part of a CI/CD pipeline.
4. Safe - scans your .tf files. Does not change any files without your permission.

# Usage
```console
tfridge <path>
```