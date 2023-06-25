# Contributing to actionslog

## Scripts

actionslog uses a number of scripts to automate common tasks. They are found in the
`script` directory.

<!--- start script descriptions --->

### bindown

script/bindown runs bindown with the given arguments

### cibuild

script/cibuild is run by CI to test this project. It can also be run locally.

### fmt

script/fmt formats go code and shell scripts.

### generate

script/generate runs all generators for this repo.
`script/generate --check` checks that the generated files are up to date.

### lint

script/lint runs linters on the project.

### release

script/release creates a new release. It is run by GitHub Actions on push to main.

### test

script/test runs tests on the project.

### update-docs

script/generate-readme updates documentation.
- For projects with binaries, it updates the usage output in README.md.
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
