# CLAUDE.md

## Pull Requests

- Title format: `type(scope)!: [level] subject` (conventional commit; subject starts with exactly one of `[high]`, `[medium]`, `[low]`). See [README.md#contributing](README.md#contributing).
- `[high]`: consensus-critical code (erasure coding/codecs, extended data square construction, repair, root computation), security, or other significant risk. `[medium]`: limited-impact code changes. `[low]`: docs, tests, or tooling.
- Use the higher level when unsure. Include the tag in the description with a brief reason, e.g. `Review level: [low] — documentation only.`

## Security PRs

- For PRs that resolve Hacken bug bounty reports, do not include details about the bug in the PR description. Instead, link to a Linear issue that contains more details on the bug and the link to the Hacken bug bounty report.
