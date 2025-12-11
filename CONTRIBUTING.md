# Contributing to kubectl-rltop

Thank you for your interest in contributing to kubectl-rltop! This document provides guidelines and instructions for contributing.

## Code of Conduct

This project adheres to a code of conduct that all contributors are expected to follow. Please be respectful and considerate of others.

## How to Contribute

### Reporting Bugs

If you find a bug, please open an issue with:
- A clear description of the problem
- Steps to reproduce the issue
- Expected behavior
- Actual behavior
- Your environment (OS, Go version, Kubernetes version)
- Any relevant logs or error messages

### Suggesting Enhancements

Enhancement suggestions are welcome! Please open an issue with:
- A clear description of the enhancement
- Use cases and examples
- Any potential implementation ideas

### Pull Requests

1. **Fork the repository** and create a branch from `main`
2. **Make your changes** following the coding standards below
3. **Add tests** if applicable
4. **Update documentation** if needed
5. **Ensure all checks pass** (tests, linting, etc.)
6. **Submit a pull request** with a clear description

#### Pull Request Guidelines

- Keep PRs focused and small when possible
- Write clear commit messages
- Reference related issues in your PR description
- Ensure CI checks pass
- Request review from maintainers

## Development Setup

### Prerequisites

- Go 1.25 or later
- kubectl configured with access to a Kubernetes cluster
- metrics-server installed in your cluster (for testing)

### Getting Started

1. Fork and clone the repository:
   ```bash
   git clone https://github.com/YOUR_USERNAME/kubectl-rltop.git
   cd kubectl-rltop
   ```

2. Install dependencies:
   ```bash
   make deps
   ```

3. Build the project:
   ```bash
   make build
   ```

4. Run tests:
   ```bash
   make test
   ```

5. Run linters:
   ```bash
   make lint
   ```

## Coding Standards

### Go Style

- Follow the [Effective Go](https://go.dev/doc/effective_go) guidelines
- Use `gofmt` to format code
- Follow the project's `.golangci.yml` configuration
- Write clear, self-documenting code
- Add comments for exported functions and types

### Code Organization

- Keep functions focused and small
- Use meaningful variable and function names
- Handle errors explicitly
- Add unit tests for new functionality

### Testing

- Write tests for new features
- Aim for good test coverage
- Use table-driven tests when appropriate
- Test error cases as well as success cases

## Commit Messages

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

- `feat:` for new features
- `fix:` for bug fixes
- `docs:` for documentation changes
- `style:` for formatting changes
- `refactor:` for code refactoring
- `test:` for adding or updating tests
- `chore:` for maintenance tasks

Example:
```
feat: add support for container-level resource display
```

## Release Process

Releases are managed by maintainers using semantic versioning:
- `MAJOR.MINOR.PATCH` (e.g., `1.2.3`)
- Major: breaking changes
- Minor: new features (backward compatible)
- Patch: bug fixes (backward compatible)

## Questions?

If you have questions, please:
- Open an issue for discussion
- Check existing issues and pull requests
- Review the documentation

Thank you for contributing to kubectl-rltop!

