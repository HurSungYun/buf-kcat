# Release Process

This document describes how to release a new version of buf-kcat.

## Prerequisites

1. Ensure all changes are committed and pushed
2. Ensure CI passes on main branch
3. Have GitHub CLI (`gh`) installed and authenticated
4. Have GoReleaser installed locally (optional, for testing)

## Release Steps

### 1. Test the Release Locally (Optional)

```bash
# Test the release process without publishing
make release-dry

# Create a snapshot release
make release-snapshot
```

### 2. Create and Push a Tag

```bash
# Create a new tag (replace v1.0.0 with your version)
VERSION=v1.0.0 make tag

# Push the tag to GitHub (this triggers the release workflow)
git push origin v1.0.0
```

### 3. Monitor the Release

1. Go to [GitHub Actions](https://github.com/HurSungYun/buf-kcat/actions)
2. Watch the "Release" workflow
3. Once complete, check the [Releases page](https://github.com/HurSungYun/buf-kcat/releases)

## Release Automation

The release process is automated using:

- **GoReleaser**: Builds cross-platform binaries, creates GitHub releases, and generates changelogs
- **GitHub Actions**: Triggers on version tags (v*) and runs the release workflow
- **Homebrew**: Automatically updates the Homebrew tap (requires HOMEBREW_TAP_TOKEN secret)

## Version Naming

Follow semantic versioning:
- `v1.0.0` - Major release (breaking changes)
- `v1.1.0` - Minor release (new features)
- `v1.1.1` - Patch release (bug fixes)

## Secrets Required

The following secrets need to be configured in GitHub repository settings:

- `HOMEBREW_TAP_TOKEN`: Personal access token with repo scope for updating the Homebrew tap (optional)

## Troubleshooting

If the release fails:

1. Check the GitHub Actions logs
2. Fix any issues
3. Delete the tag locally and remotely:
   ```bash
   git tag -d v1.0.0
   git push origin :refs/tags/v1.0.0
   ```
4. Create and push the tag again