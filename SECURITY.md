# Security Policy

## Supported versions

eme is pre-1.0; security fixes land on the latest `main` and the most recent release.

| Version | Supported |
|---------|-----------|
| latest `main` / newest release | ✅ |
| older releases | ❌ |

## Reporting a vulnerability

Please report security issues privately — **not** in a public issue.

- Use GitHub's [private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability)
  ("Report a vulnerability" on the repository's **Security** tab), or
- email **jingmuio@gmail.com** with details and steps to reproduce.

You can expect an acknowledgement within a few days. eme runs local tmux/git commands on
your machine and exposes no network service of its own, so most issues will concern unsafe
command construction or state-file handling. Please include the eme version
(`eme --version`), your OS, and your tmux/git versions.
