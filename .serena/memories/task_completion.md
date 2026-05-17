When finishing a task in this repo, prefer validating the smallest affected surface first and then the CI-shaped command if needed.

For CLI-only changes, run the relevant `cli` tests or `opctl run test` / `opctl run compile` from `cli/` when feasible. For repo-wide changes, CI indicates the important checks are changelog validation, compile, and test via `opctl run ...` tasks. If a task affects release flow or changelog handling, include `opctl run changelog/lint` and `opctl run changelog/find-in-diff` as appropriate.

Because the repo uses Ginkgo-based Go suites, test execution may happen through suite runners or IDE configurations rather than only direct `go test` usage.