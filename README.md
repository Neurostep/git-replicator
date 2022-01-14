# git-replicator

A CLI tool for git state replication from one repository to another one.

## Installation

Install it with a simple command:

```sh
go install github.com/Neurostep/git-replicator@latest
```

# Usage

```sh
usage: git-replicator <optional url>

Options:
  -b string
    	Repository branch name to replicate from (default "main")
  -l string
    	Path to local repository to replicate from
  -n int
    	Number of commits to replicate
  -r string
    	Repository remote (default "origin")
```

To authenticate requests to remote git repositories`git-replicator` uses `GIT_AUTH_TOKEN` environment variable.

To set `GIT_AUTH_TOKEN` environment variable:

```sh
$ export GIT_AUTH_TOKEN=Abc123Xyz
```

## Examples

### Replicate commits from github pull request

```sh
git-replicator <link-to-github-pull-request>
```

### Replicate commits from remote git repository by specifying URL

```sh
git-replicator <link-to-git-repository>
```

### Replicate commits from the local repository and specified branch

```sh
git-replicator -l <path-to-local-repository> -b <branch-name>
```

## How it works

If the remote repository URL is provided, `gir-replicator` will try to clone the repo locally into the directory
specified in environment variable `GITREPLICATOR_HOME`, or by default in `~/.gitreplicator`.
For the local repository, it will get the necessary data from the specified local repository.

Next, `git-replicator` will ask for a confirmation of applying the number of commits. If GitHub Pull Request
URL was provided, it will try to replicate the number of commits from the Pull Request. Otherwise, it will
use either default number of commits (`5`), or user specified (`-n <number-of-commits-to-replicate>`).

To control, which commits to pick and which to drop, `git-replicator` uses the following format:

```sh
git-replicator <github-pull-request-url>
We are about to replicate 1 commits, proceed? yes / no? no

pick 35e752f27e16e16a4d74aee7eb96f21f894b8139 Test PR

Commands:
pick <commit> = use commit
drop <commit> = remove commit
```

`git-replicator` will try to use editor specified in either `GIT_EDITOR` or `EDITOR` environment variables.
Otherwise, by default, it will try to use `vi` as editor.

If the particular patch couldn't be applied, `git-replicator` will suggest editing `patch` file, otherwise
the patch will be skipped.

## License

MIT licensed. See the [LICENSE](LICENSE) file for details.
