# AI Library Improvement Plan

## Overview

This document tracks the systematic improvements to the `github.com/liuzl/ai` library based on the comprehensive code analysis. The improvements are prioritized by impact and dependencies.

**Current Status**: In Progress (2/7 completed)
**Start Date**: 2025-11-10
**Target Completion**: TBD
**Current Test Coverage**: 65.9%
**Target Test Coverage**: 80%+

---

## Priority 1: Fix Concurrency Safety Issues

**Status**: 🟢 Completed
**Priority**: CRITICAL
**Actual Effort**: 2 hours
**Completion Date**: 2025-11-10
**Files Affected**: `tool_server.go`, `tool_server_test.go`

### Problem
- `ToolServerManager.clients` map has no concurrent access protection
- Risk of panic or data races under concurrent usage
- Maps in Go are not thread-safe

### Objectives
- Add `sync.RWMutex` to `ToolServerManager` struct
- Protect all map operations (read and write)
- Use `RLock()` for read operations, `Lock()` for write operations

### Implementation Tasks
- [x] Add `mu sync.RWMutex` field to `ToolServerManager` struct
- [x] Protect `AddRemoteServer()` with write lock
- [x] Protect `GetClient()` with read lock
- [x] Protect `ListServerNames()` with read lock
- [x] Protect `LoadFromFile()` internal map access with write lock
- [x] Add race detector tests (`go test -race`)

### Acceptance Criteria
- [x] All map operations are protected by mutex
- [x] `go test -race` passes without warnings
- [x] No breaking API changes
- [x] Existing tests still pass

### Testing Plan
- Run all tests with race detector: `go test -race ./...`
- Add concurrent access test demonstrating thread safety
- Verify performance impact is negligible

---

## Priority 2: Add Configuration Validation

**Status**: 🟢 Completed
**Priority**: HIGH
**Actual Effort**: 3 hours
**Completion Date**: 2025-11-10
**Files Affected**: `ai.go`, `config_validation_test.go`

### Problem
- No validation of model names
- No validation of baseURL format
- Timeout can be set to invalid values (e.g., negative)
- Invalid configurations cause runtime errors

### Objectives
- Validate all configuration parameters at construction time
- Return clear errors for invalid configurations
- Prevent runtime panics due to bad configuration

### Implementation Tasks
- [x] Add `validateConfig()` function in `ai.go`
- [x] Validate `timeout > 0` in `NewClient()`
- [x] Validate `baseURL` is valid URL format (if provided)
- [x] Validate `apiKey` is not empty
- [x] Validate `provider` is one of supported providers
- [x] Validate `model` is not whitespace-only if provided
- [x] Validation automatically applies via `NewClientFromEnv()`
- [x] Added 24 comprehensive test cases

### Acceptance Criteria
- [x] Invalid timeout returns error
- [x] Invalid baseURL returns error
- [x] Empty API key returns error
- [x] Unknown provider returns error with helpful message
- [x] All validation errors are clear and actionable
- [x] Existing valid configurations still work

### Testing Plan
- Add `TestConfigValidation` with subtests for each validation
- Test edge cases: zero timeout, negative timeout, malformed URLs
- Test environment variable validation
- Verify error messages are user-friendly

---

## Priority 3: Define Custom Error Types

**Status**: 🔴 Not Started
**Priority**: HIGH
**Estimated Effort**: 4-5 hours
**Files Affected**: `errors.go` (new), `http_client.go`, all adapters

### Problem
- No way to distinguish between error types programmatically
- Difficult to implement proper error handling and retries
- All errors are generic `error` type

### Objectives
- Create custom error types for different failure modes
- Enable error type assertions with `errors.As()`
- Improve error messages with structured data
- Support better retry logic based on error type

### Implementation Tasks
- [ ] Create new `errors.go` file
- [ ] Define base error interface with common methods
- [ ] Define `AuthenticationError` (401, 403)
- [ ] Define `RateLimitError` (429) with retry-after info
- [ ] Define `InvalidRequestError` (400) with details
- [ ] Define `NetworkError` for connection failures
- [ ] Define `TimeoutError` for timeout scenarios
- [ ] Define `ServerError` (5xx)
- [ ] Update `httpClient.doRequest()` to return typed errors
- [ ] Update all adapters to return typed errors
- [ ] Add error unwrapping support for error chains

### Error Type Hierarchy
```
ErrorWithStatus (interface)
├── AuthenticationError (401, 403)
├── RateLimitError (429)
├── InvalidRequestError (400)
├── ServerError (5xx)
├── NetworkError (connection failures)
└── TimeoutError (context deadline exceeded)
```

### Acceptance Criteria
- [ ] All HTTP errors return appropriate typed error
- [ ] Errors can be checked with `errors.As()`
- [ ] Error messages include relevant context (status code, provider, etc.)
- [ ] Backward compatible with existing error handling
- [ ] Documentation updated with error handling examples

### Testing Plan
- Add `TestErrorTypes` for each error type
- Test error unwrapping with `errors.As()`
- Test error messages contain expected information
- Add example of error handling in README

---

## Priority 4: Increase Test Coverage

**Status**: 🔴 Not Started
**Priority**: HIGH
**Estimated Effort**: 8-10 hours
**Files Affected**: `tool_server_test.go` (new), `http_client_test.go` (new), `ai_test.go`

### Problem
- Current coverage: 58.7%
- `tool_server.go` has no tests
- HTTP retry logic not fully tested
- Edge cases and error paths not covered

### Objectives
- Increase coverage to 80%+
- Add comprehensive tests for `tool_server.go`
- Test all retry scenarios in `http_client.go`
- Cover edge cases and error handling paths

### Implementation Tasks

#### Tool Server Tests (`tool_server_test.go`)
- [ ] Test `NewToolServerManager()`
- [ ] Test `LoadFromFile()` with valid config
- [ ] Test `LoadFromFile()` with invalid JSON
- [ ] Test `LoadFromFile()` with missing file
- [ ] Test `AddRemoteServer()`
- [ ] Test `GetClient()` for existing server
- [ ] Test `GetClient()` for non-existent server
- [ ] Test `ListServerNames()`
- [ ] Test `ToolServerClient.Connect()`
- [ ] Test `ToolServerClient.FetchTools()`
- [ ] Test `ToolServerClient.ExecuteTool()`
- [ ] Test `ToolServerClient.Close()`
- [ ] Test lazy connection on first use
- [ ] Test concurrent access to manager

#### HTTP Client Tests (`http_client_test.go`)
- [ ] Test successful request (no retry)
- [ ] Test retry on 500 error
- [ ] Test retry on 503 error
- [ ] Test no retry on 400 error
- [ ] Test no retry on 401 error
- [ ] Test max retries exceeded
- [ ] Test exponential backoff timing
- [ ] Test jitter is applied
- [ ] Test context cancellation during retry
- [ ] Test timeout during request
- [ ] Test network error retry

#### Additional Coverage (`ai_test.go`)
- [ ] Test `NewClientFromEnv()` with missing env vars
- [ ] Test `NewClientFromEnv()` with invalid provider
- [ ] Test context cancellation in `Generate()`
- [ ] Test empty message handling
- [ ] Test tool call with invalid JSON arguments
- [ ] Test response with malformed JSON

### Acceptance Criteria
- [ ] Test coverage >= 80%
- [ ] All new tests pass
- [ ] Tests are fast (< 5 seconds total)
- [ ] Tests are deterministic (no flaky tests)
- [ ] Mock external dependencies (no real API calls)

### Testing Plan
- Run `go test -cover` and verify coverage increase
- Run `go test -race` to check for race conditions
- Run tests multiple times to verify determinism
- Add coverage badge to README

---

## Priority 5: Eliminate Code Duplication

**Status**: 🔴 Not Started
**Priority**: MEDIUM
**Estimated Effort**: 3-4 hours
**Files Affected**: `client_*.go`, `format_service.go`, adapters

### Problem
- Client constructors are nearly identical across providers
- Type definitions duplicated between adapters and `format_service.go`
- Tool conversion logic duplicated

### Objectives
- Extract common client construction logic
- Share type definitions across files
- Reduce maintenance burden

### Implementation Tasks
- [ ] Create `newProviderClient()` helper function
- [ ] Refactor `NewOpenAIClient()` to use helper
- [ ] Refactor `NewGeminiClient()` to use helper
- [ ] Refactor `NewAnthropicClient()` to use helper
- [ ] Move shared types to common file (e.g., `types.go`)
- [ ] Update `format_service.go` to import shared types
- [ ] Remove duplicate type definitions
- [ ] Extract common tool conversion logic

### Acceptance Criteria
- [ ] No functional changes (all tests pass)
- [ ] Reduced lines of code
- [ ] Single source of truth for types
- [ ] Easier to add new providers

### Testing Plan
- Run all existing tests to ensure no regressions
- Verify code coverage remains the same or improves
- Check that godoc output is still clear

---

## Priority 6: Add Structured Logging Support

**Status**: 🔴 Not Started
**Priority**: MEDIUM
**Estimated Effort**: 4-5 hours
**Files Affected**: `logger.go` (new), `ai.go`, `http_client.go`, `tool_server.go`

### Problem
- No logging capability
- Difficult to debug issues in production
- No visibility into retry attempts, API calls, etc.

### Objectives
- Define optional logger interface
- Add logging at key points (requests, retries, errors)
- Support common logging libraries (slog, zap, logrus)
- No logging by default (backward compatible)

### Implementation Tasks
- [ ] Create `logger.go` with `Logger` interface
- [ ] Define minimal interface: `Debug()`, `Info()`, `Warn()`, `Error()`
- [ ] Add `WithLogger()` functional option
- [ ] Add logger field to `httpClient`
- [ ] Log HTTP requests (method, URL, provider)
- [ ] Log retry attempts with backoff duration
- [ ] Log errors with context
- [ ] Add logger to `ToolServerClient`
- [ ] Log tool server connections, disconnections
- [ ] Log tool executions
- [ ] Create adapter for stdlib `slog`
- [ ] Add logging example to examples/

### Logger Interface
```go
type Logger interface {
    Debug(msg string, keysAndValues ...interface{})
    Info(msg string, keysAndValues ...interface{})
    Warn(msg string, keysAndValues ...interface{})
    Error(msg string, keysAndValues ...interface{})
}
```

### Acceptance Criteria
- [ ] Logger is optional (nil logger = no logging)
- [ ] No performance impact when logger is nil
- [ ] Sensitive data (API keys) are not logged
- [ ] Log messages are structured (key-value pairs)
- [ ] Example demonstrates logging integration

### Testing Plan
- Test with nil logger (no panics)
- Test with mock logger (verify log calls)
- Verify API keys are redacted
- Performance benchmark with/without logging

---

## Priority 7: Add Request Validation

**Status**: 🔴 Not Started
**Priority**: MEDIUM
**Estimated Effort**: 3-4 hours
**Files Affected**: `ai.go`, `provider_adapter.go`

### Problem
- No validation of `Request` fields
- Empty messages allowed
- Invalid tool definitions accepted
- Errors only discovered when API call fails

### Objectives
- Validate requests before sending to API
- Provide clear error messages for invalid inputs
- Prevent wasted API calls
- Improve developer experience

### Implementation Tasks
- [ ] Create `Request.Validate()` method
- [ ] Check `Messages` is not empty
- [ ] Check each message has content or tool calls
- [ ] Check message roles are valid
- [ ] Check tool definitions have required fields (name, parameters)
- [ ] Check tool parameters are valid JSON Schema
- [ ] Check model name is not empty (if specified)
- [ ] Add validation call in `genericClient.Generate()`
- [ ] Add helpful error messages for each validation failure

### Validation Rules
- `len(Messages) > 0`
- Each message must have: valid `Role`, non-empty `Content` OR `ToolCalls`
- Tool role messages must have `ToolCallID`
- Each tool must have: non-empty `Name`, non-empty `Type`, valid `Parameters` schema
- `Model` if specified must be non-empty string

### Acceptance Criteria
- [ ] Empty request returns validation error
- [ ] Invalid message roles return error
- [ ] Invalid tool definitions return error
- [ ] Valid requests pass validation
- [ ] Error messages clearly explain what's wrong
- [ ] Validation is fast (< 1ms)

### Testing Plan
- Add `TestRequestValidation` with subtests
- Test each validation rule
- Test valid requests pass
- Verify error messages are actionable
- Benchmark validation performance

---

## Progress Tracking

### Completion Status

| Priority | Item | Status | Completion Date | Notes |
|----------|------|--------|-----------------|-------|
| 1 | Concurrency Safety | 🟢 Completed | 2025-11-10 | ✅ All tests pass with -race |
| 2 | Config Validation | 🟢 Completed | 2025-11-10 | ✅ 24 test cases, clear errors |
| 3 | Error Types | 🔴 Not Started | - | Better error handling |
| 4 | Test Coverage | 🔴 Not Started | - | Quality assurance |
| 5 | Code Duplication | 🔴 Not Started | - | Maintainability |
| 6 | Structured Logging | 🔴 Not Started | - | Observability |
| 7 | Request Validation | 🔴 Not Started | - | Developer experience |

### Legend
- 🔴 Not Started
- 🟡 In Progress
- 🟢 Completed
- ⏸️ Blocked/Paused

---

## Guidelines

### Process for Each Item

1. **Update Status**: Mark item as 🟡 In Progress
2. **Implementation**: Complete all tasks in checklist
3. **Testing**: Ensure all acceptance criteria met
4. **Code Review**: Self-review changes
5. **Documentation**: Update relevant docs
6. **Commit**: Create focused git commit
7. **Update Status**: Mark as 🟢 Completed with date

### Commit Message Format

```
<type>(<scope>): <description>

<body explaining what and why>

Addresses: TODO.md Priority <N>
```

Example:
```
fix(tool_server): Add mutex for concurrent access safety

- Add sync.RWMutex to ToolServerManager
- Protect all map operations
- Add race detector test

Addresses: TODO.md Priority 1
```

### Testing Requirements

- All new code must have tests
- Tests must pass with race detector: `go test -race`
- Coverage should not decrease
- No flaky tests allowed

### Breaking Changes

- Avoid breaking changes when possible
- If unavoidable, document clearly
- Consider deprecation period
- Update examples and README

---

## Future Enhancements (Beyond Current Scope)

These items are identified but not in current plan:

- Streaming response support
- Multimodal inputs (images, audio, video)
- Request/response caching
- Middleware/interceptor hooks
- Connection pooling for tool servers
- Metrics/telemetry integration
- Circuit breaker pattern
- Request batching
- Response pagination handling

---

## References

- **Analysis Document**: See comprehensive analysis from 2025-11-10
- **Test Coverage Report**: Run `go test -cover` for current metrics
- **Race Detector**: Use `go test -race` to detect concurrency issues
- **Go Best Practices**: https://go.dev/doc/effective_go

---

**Last Updated**: 2025-11-10
**Owner**: @liuzl
**Reviewers**: TBD
