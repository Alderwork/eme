# Contributing to eme

Thank you for your interest in contributing!

By participating, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).
CI runs `go build`, `go vet`, and `go test` on every push and pull request.

## Getting started

1. Fork the repository.
2. Clone your fork.
3. Run `go build ./...` to verify the build.
4. Run `go test ./...` to verify tests.

## Pull requests

- Open an issue first for large changes.
- Keep changes focused and minimal.
- Add tests for new functionality.
- Update relevant documentation.

## Code style

- Follow standard Go conventions.
- Avoid `any` for type declarations; prefer concrete interfaces.
- Keep error messages actionable and conversational.

## Reporting bugs

Use the bug report issue template and include your environment details.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
