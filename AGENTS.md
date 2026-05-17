# Repository Guidelines

## Project Structure & Module Organization

This is a monorepo with one Go module at the root. `cli/` contains the
Cobra-based command line app and embeds `webapp/` at runtime. `sdks/go/` and
`sdks/js/` hold the Go and TypeScript SDKs. `api/` contains the OpenAPI spec,
`opspec/opfile/` contains the opfile JSON schema, and `test-suite/` contains
end-to-end scenario ops. `webapp/` is the React app, `website/` is the
Docusaurus site, and `.opspec/` plus per-project `.opspec/` directories define
build, test, format, and release workflows.

## Build, Test, and Development Commands

- `opctl run -a version=0.0.0 compile`: builds releasable artifacts, including the webapp and CLI.
- `opctl run -a githubAccessToken=$TOKEN test`: runs API, CLI unit/e2e,
  opspec, webapp, Go SDK, and JS SDK tests. The token is required for
  authenticated test-suite scenarios.
- `opctl run format`: runs `go fmt` for `./sdks/go/...` and `./cli/...`.
- `go build -o opctl-beta ./cli`: builds a local CLI binary for native debugging.
- From subprojects, use local ops such as `cd cli && opctl run test`, `cd webapp && opctl run dev`, or `cd website && opctl run dev`.

## Coding Style & Naming Conventions

Use the root `.editorconfig`: UTF-8, LF line endings, final newline, two-space
default indentation. Go code must be `gofmt` formatted and follow Go Code
Review Comments. Keep Go packages lower-case and tests beside source as
`name_test.go`. JavaScript and TypeScript areas follow StandardJS style. React
code should use functional components and hooks; prefer style objects, using
Emotion-generated class names when needed.

## Testing Guidelines

Go tests use Ginkgo/Gomega. Write tests in arrange, act, assert form and name
the subject `objectUnderTest`. Keep dependency interactions behind interfaces
and use Counterfeiter fakes where appropriate. JS/TS tests live next to source
as `code.test.ts`. End-to-end CLI coverage lives under `test-suite/`; each
scenario directory is an op with `scenarios.json`.

## Commit & Pull Request Guidelines

Recent commits use short imperative subjects such as `Fix git_test.go marker
path computation` or `Bump github.com/go-git/go-git/v5`. Keep subjects specific
and scoped. Pull requests need maintainer approval, a passing GitHub Actions
Build workflow, and a `CHANGELOG.md` entry when applicable. Include the problem,
approach, test results, and linked issue; add screenshots only for visible
webapp or website changes.

## Security & Configuration Tips

Do not commit secrets. Pass sensitive values through op inputs such as
`githubAccessToken`. Local Go development requires Go from `go.mod`; macOS
contributors may also need `gpgme` via `brew install gpgme`.

@RTK.md
