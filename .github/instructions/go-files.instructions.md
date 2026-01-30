---
applyTo: '**/*.go'
---

# Go Code Instructions

This repository uses Go 1.25.6+ with specific coding standards and best practices.

## Go Version Requirements:

- **Minimum version**: Go 1.25.6
- **Target version**: Go 1.25.6
- **Compatibility**: Ensure code works with Go 1.25.6 features

## Code Quality Standards:

- Use modern Go idioms and patterns
- Follow standard Go formatting (gofmt)
- Include proper error handling with error wrapping
- Use generics where appropriate for type safety
- Ensure proper documentation for exported functions and types
- Follow Go naming conventions (camelCase for private, PascalCase for exported)
- Use structured logging through internal/logging package
- Keep public structs JSON-tagged for CLI/JSON outputs

## Security Best Practices:

- Validate all inputs and handle edge cases
- Use proper error wrapping and context propagation
- Avoid common security vulnerabilities
- Follow Go security guidelines

## Performance Considerations:

- Use appropriate data structures for the use case
- Consider memory usage and garbage collection impact
- Use goroutines and channels appropriately
- Profile performance-critical code

## Testing Requirements:

- Write comprehensive unit tests
- Use table-driven tests where appropriate
- Test both success and error paths
- Include edge cases and boundary conditions
- Mock external dependencies appropriately
- Aim for high test coverage (target: 80%+)

## Import Organization:

- Group imports: standard library, third-party, internal
- Use goimports for automatic import organization
- Avoid unused imports
- Use proper import aliases when needed

## Error Handling:

- Return errors instead of panicking
- Use error wrapping for context preservation
- Handle errors at appropriate levels
- Provide meaningful error messages

## Logging:

- Use structured logging through internal/logging
- Include trace IDs for request correlation
- Log at appropriate levels (DEBUG, INFO, WARN, ERROR)
- Include relevant context in log messages
