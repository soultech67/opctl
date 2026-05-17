# Changelog Policy

For this repository, every workbranch task that changes repository state must include a `CHANGELOG.md` update before the work is considered complete.

When adding an entry:
- Find the latest release tag, e.g. `git describe --tags --abbrev=0`.
- Use exactly the next patch version from that tag for the workbranch's changelog entry. Example: latest tag `v0.1.76` means the branch should use `## 0.1.77`.
- Do not skip ahead more than one patch version, and do not create multiple future versions on the same branch.
- If the correct next patch section already exists, append/update bullets in that section rather than adding another version.
- Summarize all meaningful branch changes, including CI, release, docs, tests, and behavior changes.
- Re-check this before committing or finalizing work on any branch.

Historical changelog links can remain pointed at their original upstream issues/PRs unless the task explicitly asks to rewrite history.