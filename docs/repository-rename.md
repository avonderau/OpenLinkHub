# Repository rename: OpenLinkHub-Nexus

The GitHub repository for this independently maintained fork is
[avonderau/OpenLinkHub-Nexus](https://github.com/avonderau/OpenLinkHub-Nexus).
The software and service remain named **OpenLinkHub Nexus**. Release tags keep
the upstream version with the fork's **N** suffix; existing `0.9.4N`
artifacts are not rebuilt or retagged by this change.

## Repository owner's GitHub step

The repository owner must rename the repository in the GitHub web interface:

1. Open the old repository and select **Settings → General**.
2. Under **Repository name**, enter `OpenLinkHub-Nexus` and confirm **Rename**.
3. Verify that `https://github.com/avonderau/OpenLinkHub-Nexus` opens the
   repository and that `/releases/tag/0.9.4N` retains the DEB/RPM/tarball assets.

The GitHub App connection used to maintain this fork does not have repository
administration permission and cannot perform the rename itself.

## Local development checkouts

A local Git clone will generally follow GitHub's redirect from the old remote,
but updating `origin` avoids relying on it:

```bash
cd /path/to/your/local/OpenLinkHub
git remote set-url origin https://github.com/avonderau/OpenLinkHub-Nexus.git
git remote -v
git fetch origin
```

These commands change only the Git remote; they do **not** delete local
changes or update a running installation. The compiled service and
`/opt/OpenLinkHub` installation paths are not renamed.

## Follow-up verification

- Sidebar **GitHub** link, release documentation, and badges use the new URL.
- Remote installer uses the new repository's GitHub Releases API.
- Release workflows use the renamed repository identity, not the old slug.
- Native DEB and RPM package metadata links point to the renamed repository.
- Check GitHub Actions and the existing `0.9.4N` release after rename.
  The native-package job can be manually run from Actions to refresh the
  published release notes; it preserves existing release assets.
- GitHub redirects repository and release URLs, but external webhooks, app
  connections, package/container consumers, and GitHub Container Registry
  paths may require separate updates. Docker image names based on
  `github.repository` change to the new repository path for future builds.
- Original upstream project credit and fork relationship are retained.
