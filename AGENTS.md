## Build / Test commands
- go test ./...
- go test -run TestGolden -v ./...

## Repo rules
- Tests must be deterministic (no network, no time-dependent outputs).
- Do not add large binaries to git (pcapは testdata に最小限、必要なら圧縮/分割).
- Prefer small PRs (<= 300 lines changed) unless unavoidable.

## Review guidelines
- Verify golden test outputs are stable and human-readable.
- Ensure new parsers fail safely on unknown PFCP IEs (no panic).
