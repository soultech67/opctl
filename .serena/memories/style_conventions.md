Formatting is governed by `.editorconfig`: UTF-8, LF line endings, spaces for indentation, and default indent size 2. The repository is a Go monorepo, so idiomatic Go naming and package layout conventions apply.

For CLI tests, `cli/CONTRIBUTING.md` specifies: write tests in arrange/act/assert form, refer to the subject as `objectUnderTest`, keep tests adjacent to source files (`code_test.go` next to `code.go`), and depend on interfaces rather than concrete implementations. Fakes are generated and used for dependency interaction tests.

Testing uses Ginkgo/Gomega in many Go packages, so test structure often follows suite-based patterns rather than plain `go test` conventions alone.