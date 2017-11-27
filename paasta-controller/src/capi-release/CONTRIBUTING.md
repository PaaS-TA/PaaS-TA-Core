# Contributing to CAPI Release

The Cloud Foundry team uses GitHub and accepts contributions via
[pull request](https://help.github.com/articles/using-pull-requests).

## Contributor License Agreement

Follow these steps to make a contribution to any of our open source repositories:

1. Ensure that you have completed our CLA Agreement for
  [individuals](https://www.cloudfoundry.org/pdfs/CFF_Individual_CLA.pdf) or
  [corporations](https://www.cloudfoundry.org/pdfs/CFF_Corporate_CLA.pdf).

1. Set your name and email (these should match the information on your submitted CLA)

        git config --global user.name "Firstname Lastname"
        git config --global user.email "your_email@example.com"

## General Workflow

1. Fork the repository
1. Check out `develop` of capi-release
1. Create a feature branch (`git checkout -b <my_new_branch>`)
1. Make changes on your branch
1. Deploy your changes to your dev environment and run [CF Acceptance Tests (CATS)](https://github.com/cloudfoundry/cf-acceptance-tests/)
1. Push to your fork (`git push origin <my_new_branch>`) and submit a pull request

We favor pull requests with very small, single commits with a single purpose.

Your pull request is much more likely to be accepted if it is small and focused with a clear message that conveys the intent of your change.
