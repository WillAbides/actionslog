# Contributing to actionslog

Your issues and pull requests are welcome. If you have a non-trivial change, you may want to open an issue first so we
can discuss whether it will be a good fit before you spend time on it. Don't let that stop you from coding, though. Just
be aware that it may not get merged into this repo.

## Generating Code

We use code generation to support both "log/slog" and "golang.org/x/exp/slog". We lovingly handcraft code for "log/slog"
and then generate the "golang.org/x/exp/slog" version. Run `script/generate` before you commit to make sure the
generated code is up-to-date. CI is happy to remind you if you forget.

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

Releases are automated with [release-train](https://github.com/WillAbides/release-train). Every pull request needs a
label indicating the change level. Add whatever label you think fits, but don't spend too much time thinking it over. We
will adjust the label as needed before merging.

| Label           | Change Level |
|-----------------|--------------|
| semver:breaking | major        |
| semver:minor    | minor        |
| semver:patch    | patch        |
| semver:none     | none         |
