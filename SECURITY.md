# Security policy

lazygram holds a Telegram session, an encrypted message cache and the
attachments you open, so a security bug in it can expose a whole account.
Reports are welcome and handled privately.

## Reporting a vulnerability

Report it privately through GitHub:
https://github.com/t0d0r/lazygram/security/advisories/new (the
Security tab, "Report a vulnerability"). Please do not open a public
issue, discussion or pull request for a vulnerability.

Include the version (`lazygram -version`), the operating system and
terminal, the steps to reproduce and what an attacker gains. Never send
your Telegram session, the state directory, credentials, phone numbers
or private messages: describe them or redact them; invented data from
`lazygram demo` is enough to show most problems.

## What happens next

- An acknowledgement within 7 days.
- An assessment within 14 days: whether it is a vulnerability, how
  severe, and the plan.
- A fixed release within 90 days of the report, sooner for a severe
  one. The fix is disclosed in a GitHub security advisory (with a CVE
  when warranted) once the release is out; the reporter is credited
  unless they ask not to be.
- If no fix is ready after 90 days, the disclosure date is agreed with
  the reporter.

## Supported versions

Only the latest release gets fixes; `lazygram update` or `brew
upgrade` installs it. An Arch or Omarchy package built with `makepkg
-si` from `packaging/omarchy/lazygram` is updated by pulling this
repository and running `makepkg -si` again once the PKGBUILD names the
new release (`pacman -Syu` once lazygram is in pkgs.omarchy.org).
Releases are not maintained in parallel.

## Scope

In scope: the code in this repository, the release tarballs,
`install.sh`, and the packaging recipes under `packaging/`. Examples:
terminal escape or control sequences from a message reaching the
terminal, the session or cache key leaking, cache or vault files
readable by other local users, files written outside the state
directory, a downloaded update or release that is not what this
repository's Release workflow built.

Out of scope: Telegram's servers and the MTProto protocol, bugs in
dependencies that lazygram does not reach (report those upstream; tell
us if lazygram is affected), and attacks that need control of the
user's account or machine already.

## Verifying a release

Every release from v0.4.22 on is built by the Release workflow
(`.github/workflows/release.yml`) on GitHub-hosted runners and carries
a SLSA build provenance attestation signed through Sigstore. With the
GitHub CLI:

```sh
gh attestation verify lazygram-vX.Y.Z-linux-amd64.tar.gz --repo t0d0r/lazygram \
  --signer-workflow t0d0r/lazygram/.github/workflows/release.yml \
  --source-ref refs/tags/vX.Y.Z --deny-self-hosted-runners
```

This checks that the tarball was built by that workflow in this
repository, for the tag `vX.Y.Z`, and names the commit. A plain
`--repo` check is weaker: it accepts any workflow of the repository,
from any branch. The workflow also refuses a tag whose commit is not on
`master`; that check is not recorded in the certificate.

The same attestation is attached to the release as
`lazygram-vX.Y.Z.intoto.jsonl`; add `--bundle
lazygram-vX.Y.Z.intoto.jsonl` to the command above (same flags) to
check against that file instead of GitHub's attestation store.
