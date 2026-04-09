# Contributing to DMGN

Thank you for your interest in contributing to DMGN. This document outlines the guidelines for contributing to the project.

## Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md). Please read it before contributing.

## How to Contribute

### Reporting Bugs

1. Search existing issues to avoid duplicates
2. Create a new issue with a clear title and description
3. Include reproduction steps, expected behavior, and actual behavior
4. Attach relevant logs, screenshots, or code samples

### Suggesting Features

1. Open a discussion in GitHub Discussions
2. Describe the feature and its use case
3. Explain why this feature would be beneficial
4. Be open to feedback and alternative approaches

### Pull Requests

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/your-feature-name`
3. Make your changes and add tests
4. Ensure all tests pass: `go test ./...`
5. Commit with descriptive messages
6. Push to your fork and submit a PR
7. Fill out the PR template completely

## Development Setup

### Requirements

- Go 1.21+
- Git

### Building

```bash
git clone https://github.com/nnlgsakib/dmgn
cd dmgn
go build -o dmgn ./cmd/dmgn
```

### Running Tests

```bash
go test ./...
```

### Code Style

- Run `go fmt` before committing
- Follow standard Go conventions
- Add documentation for exported functions
- Write tests for new functionality

## Commit Messages

Use clear, descriptive commit messages:

```
type(scope): description

- detail 1
- detail 2
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation
- `refactor`: Code refactoring
- `test`: Tests
- `chore`: Maintenance

## Issue Labels

| Label | Description |
|-------|-------------|
| `bug` | Bug report |
| `feature` | New feature request |
| `help wanted` | Seeking contributions |
| `good first issue` | Good for newcomers |
| `question` | Question or discussion |

## Recognition

Contributors will be listed in the [Acknowledgments](README.md#acknowledgments) section of the README.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).