# Contributing to actionslog

## Scripts

actionslog uses a number of scripts to automate common tasks. They are found in the
`script` directory.

<!--- start script descriptions --->

### bindown

script/bindown runs bindown with the given arguments

### cibuild

script/cibuild runs the same checks as GitHub Actions.

### fmt

script/fmt formats go code and shell scripts.

### generate

script/generate generates code and docs.
`script/generate --check` checks that the generated files are up to date.

### lint

script/lint lints go code and shell scripts.

### test

script/test runs go tests.

### update-docs

script/generate-readme updates documentation.
- Adds script descriptions to CONTRIBUTING.md.

<!--- end script descriptions --->

## Releasing

Releases are automated with GitHub Actions. The release workflow runs on every push to main and determines the version
to release based on the labels of the PRs that have been merged since the last release. The labels it looks for are:

| Label           | Change Level |
|-----------------|--------------|
| breaking        | major        |
| breaking change | major        |
| major           | major        |
| semver:major    | major        |
| bug             | patch        |
| enhancement     | minor        |
| minor           | minor        |
| semver:minor    | minor        |
| bug             | patch        |
| patch           | patch        |
| semver:patch    | patch        |
