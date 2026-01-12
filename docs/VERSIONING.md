# Development

## Releasing a New Version

### 1. Update version references

Update any version numbers in the codebase if needed.

### 2. Create and push a new tag

```bash
# Create an annotated tag
git tag -a v1.0.7 -m "Release v1.0.7"

# Push the tag to remote
git push origin v1.0.6
```

**Important:** This repo uses a multi-module Go setup. The `node` package at `github.com/unblink/unblink/node` is a separate module with its own go.mod. Tags like `v1.0.x` apply to both the main repo and the node module.

### 3. Users can update with

```bash
go install github.com/unblink/unblink/node/cmd/unblink@latest
```

**Note:** The Go module proxy (`proxy.golang.org`) may take 1-5 minutes to fetch new tags from GitHub. Users experiencing stale versions can:

```bash
# Clear local Go module cache
go clean -modcache

# Or bypass the proxy temporarily
GOPROXY=direct go install github.com/unblink/unblink/node/cmd/unblink@latest
```

### 4. Verify the release

```bash
# Check the tag was created
git tag -l "v*"

# Verify tag contents
git show v1.0.7

# Users can verify their version
unblink --version
```
