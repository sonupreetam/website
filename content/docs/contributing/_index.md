---
title: "Contributing"
description: "Learn how to contribute to ComplyTime projects."
lead: "Join the community and help build the future of compliance automation."
date: 2024-01-01T00:00:00+00:00
lastmod: 2024-12-24T00:00:00+00:00
draft: false
images: []
weight: 400
toc: true
---

## Welcome Contributors!

ComplyTime is an open source project and we welcome contributions of all kinds. Whether you're fixing bugs, adding features, improving documentation, or helping others in the community, your contributions are valued.

## Quick Links

- 📖 [Community Repository](https://github.com/complytime/community)
- 💬 [GitHub Discussions](https://github.com/orgs/complytime/discussions)
- 🐛 [Report Issues](https://github.com/complytime)

## Ways to Contribute

### 🐛 Report Bugs

Found a bug? Please report it on the relevant project's GitHub repository:

1. Search existing issues to avoid duplicates
2. Use the bug report template
3. Include reproduction steps
4. Provide environment details

### ✨ Suggest Features

Have an idea? We'd love to hear it:

1. Open a discussion or issue
2. Describe the use case
3. Explain the expected behavior
4. Consider implementation approaches

### 📝 Improve Documentation

Documentation improvements are always welcome:

- Fix typos and grammar
- Add examples
- Clarify explanations
- Translate content

### 💻 Submit Code

Ready to code? Here's how:

1. **Fork** the repository
2. **Create a branch** for your change
3. **Make changes** following our guidelines
4. **Write tests** for new functionality
5. **Submit a pull request**

## Development Setup

### Prerequisites

Most ComplyTime projects require:

- **Go 1.21+** for Go projects
- **Python 3.10+** for Python projects
- **Git** for version control
- **Make** for build automation

### Clone and Build

```bash
# Clone a project
git clone https://github.com/complytime/complyctl.git
cd complyctl

# Install dependencies
make deps

# Build
make build

# Run tests
make test
```

## Code Guidelines

### Go Projects

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` for formatting
- Run `golangci-lint` before submitting
- Write tests for new code
- Document exported functions

```go
// Good: Clear documentation
// ValidateControl checks if a control meets the specified requirements.
// It returns an error if validation fails.
func ValidateControl(control Control, reqs Requirements) error {
    // Implementation
}
```

### Python Projects

- Follow [PEP 8](https://pep8.org/)
- Use type hints
- Run `black` for formatting
- Run `ruff` for linting
- Write docstrings

```python
def validate_control(control: Control, requirements: Requirements) -> bool:
    """
    Check if a control meets the specified requirements.

    Args:
        control: The control to validate
        requirements: Requirements to check against

    Returns:
        True if validation passes, False otherwise
    """
    pass
```

## Pull Request Process

### Before Submitting

1. ✅ Tests pass locally
2. ✅ Code follows style guidelines
3. ✅ Documentation is updated
4. ✅ Commits are signed (DCO)

### PR Template

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Documentation update
- [ ] Refactoring

## Testing
How was this tested?

## Checklist
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] DCO sign-off
```

### Developer Certificate of Origin (DCO)

We require a DCO sign-off on all commits:

```bash
git commit -s -m "feat: add new feature"
```

This adds a `Signed-off-by` line to your commit message, certifying you have the right to submit the code.

## Community Guidelines

### Code of Conduct

We are committed to providing a welcoming and inclusive environment. Please read and follow our [Code of Conduct](https://github.com/complytime/community/blob/main/CODE_OF_CONDUCT.md).

### Communication

- Be respectful and constructive
- Assume good intentions
- Help others learn and grow
- Celebrate contributions

## Recognition

We recognize all contributors! Your contributions will be:

- Listed in release notes
- Credited in the project
- Celebrated in the community

## Getting Help

Stuck? Need help? Here's how to get support:

1. **Documentation** - Check the docs first
2. **Discussions** - Ask in GitHub Discussions
3. **Issues** - Search existing issues
4. **Community** - Reach out to maintainers

## Project Governance

ComplyTime follows an open governance model:

- **Maintainers** - Core project maintainers
- **Contributors** - Active contributors
- **Community** - All users and supporters

See the [Community Repository](https://github.com/complytime/community) for governance details.

---

Thank you for your interest in contributing to ComplyTime! Every contribution, no matter how small, helps make compliance automation better for everyone. 🎉

