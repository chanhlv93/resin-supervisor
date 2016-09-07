# Development Guide

## Building Gosuper
### Build with `go`

- you need `go v1.5` or later
- You need to set export `GO15VENDOREXPERIMENT=1` environment variable
- If your working copy is not in your `GOPATH`, you need to set it accordingly.

```console
$ cd $GOPATH/src/github.com/resin-io/resin-supervisor/
$ go install github.com/resin-io/resin-supervisor/gosuper/gosuper
```
## Workflow
### Fork the main repository

1. Go to https://github.com/deviceMP/resin-supervisor
2. Click the "Fork" button (at the top right)

### Clone your fork

The commands below require that you have $GOPATH. We highly recommended you put resin-supervisor' code into your $GOPATH.

```console
mkdir -p $GOPATH/src/github.com/resin-io
cd $GOPATH/src/github.com/resin-io
git clone https://github.com/$YOUR_GITHUB_USERNAME/resin-supervisor.git
cd resin-supervisor
git remote add upstream 'https://github.com/deviceMP/resin-supervisor'
```

### Create a branch and make changes

```console
git checkout -b myfeature
# Make your code changes
```

### Keeping your development fork in sync

```console
git fetch upstream
git rebase upstream/devel
```
### Committing changes to your fork

```console
git commit
git push -f origin myfeature
```

### Creating a pull request

1. Visit https://github.com/$YOUR_GITHUB_USERNAME/resin-supervisor.git
2. Click the "Compare and pull request" button next to your "myfeature" branch.
3. Check out the pull request process for more details
