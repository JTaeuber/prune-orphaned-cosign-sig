# prune-orphaned-cosign-sig

`prune-orphaned-cosign-sig` is a GitHub Action that removes cosign signatures from a GHCR repository when their corresponding versions no longer exists.

## ⚠️ Word of caution

As this action is destructive, it's recommended to test any changes to the configuration of the action with a dry-run to ensure the expected signatures are matched for pruning.

## Quick start

Pruning all orphaned cosign signatures for an organization:

```yml
steps:
  - name: Prune
    uses: jtaeuber/prune-orphaned-cosign-sig@v0.1.0
    with:
      gh_token: ${{ secrets.YOUR_TOKEN }}
      gh_org: your-org
      package_name: your-package
      dry-run: true # Dry-run first, then change to `false`
```

## Permissions

This action uses the Github Rest API PackageDeleteVersion() function for [users](https://docs.github.com/en/rest/packages/packages#delete-a-package-version-for-the-authenticated-user) and [orgs](https://docs.github.com/en/rest/packages/packages#delete-package-version-for-an-organization) which states:

> OAuth app tokens and personal access tokens (classic) need the read:packages and delete:packages scopes to use this endpoint.
> In addition:
> [...]
> If `package_type` is container, you must also have admin permissions to the container you want to delete.

As a result, for this action to work, the token must be associated to a user who has admin permissions for both the organization and the package. If this is not the case, then dry-runs will work as expected but actual runs will fail with a `Package not found` error when attempting to delete versions.

## Inputs

### gh_token

**Required** Secret access token with scopes `packages:read` and `packages:delete` and write permissions on the targeted container. See [Creating a personal access token
](https://docs.github.com/en/github/authenticating-to-github/keeping-your-account-and-data-secure/creating-a-personal-access-token) for more details about GitHub access tokens.

### gh_org

Name of the organization owning the container package.

:warning: This input is mutually exclusive with input `user`.
Only one of the 2 can be used at any time.
If neither are provided, then the packages of the authenticated user (cf. `gh_token`) are considered.

### gh_user

Name of the user owning the package.

:warning: This input is mutually exclusive with input `organization`.
Only one of the 2 can be used at any time.
If neither are provided, then the packages of the authenticated user (cf. `gh_token`) are considered.

### package_name

**Required** Name of the package for which signatures should be pruned.

### dry-run

**Optional** Boolean controlling whether to execute the action as a dry-run. When `true` the action will print out details of the version that will be pruned without actually deleting them. Defaults to `false`.

As this action is destructive, it's recommended to test any changes to the configuration of the action with a dry-run to ensure the expected versions are matched for pruning.
