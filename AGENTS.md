# Repository Guidelines

## Project Structure & Module Organization
- `main.go` coordinates CLI flags, configuration loading, and the scanning pipeline entrypoint.
- `pkg/config` parses options, `pkg/domain` handles host decomposition, `pkg/http` and `pkg/fasthttp` encapsulate request backends, `pkg/result` aggregates findings, and `pkg/utils` provides shared helpers.
- `input/` hosts sample wordlists (`example-paths.txt`, etc.); keep real assessment data outside the repo and document alternative locations in PRs.

## Build, Test & Development Commands
- `go build -o dynamic_file_searcher` — compile the CLI binary from the repository root.
- `go run . -domain example.com -paths input/example-paths.txt` — execute a smoke scan without writing a binary.
- `go fmt ./...` and `go vet ./...` — format code and surface obvious issues before raising a review.

## Coding Style & Naming Conventions
- Adhere to gofmt defaults (tabs for indentation, organized imports); run `gofmt -w` on touched files or enable IDE auto-formatting.
- Keep package names lowercase and concise; exported types/functions use PascalCase while internals stay camelCase.
- Place new functionality under the matching `pkg/<domain>` folder and mirror existing file naming patterns for discoverability.

## Testing Guidelines
- Create table-driven tests in `*_test.go` siblings of the code they exercise; grow shared fixtures under `testdata/` when necessary.
- Run `go test ./...` for the full suite and `go test ./pkg/domain -run TestHostDepth` to target specific logic.
- Focus coverage on path generation, marker matching, and HTTP client selection edge cases when introducing new behavior.

## Commit & Pull Request Guidelines
- Mirror the current history with concise, imperative commit titles (e.g., `add stdin support for domains`); add scoped prefixes when clarity improves.
- PR descriptions should explain the problem, summarize the solution, list new flags or defaults, and include CLI snippets or screenshots for user-visible changes.
- Reference linked issues and call out follow-up tasks (docs, larger scans, performance checks) so reviewers understand remaining work.

## Security & Configuration Tips
- Do not commit real targets or credentials; store sensitive wordlists outside `input/` and mention their secure location in PR notes.
- Tune `-concurrency`, `-timeout`, proxy flags, and marker lists thoughtfully during testing to avoid unintentional traffic spikes or noisy diffs.
