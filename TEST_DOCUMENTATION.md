# Test Documentation for Kubernetes State Monitor Operator

## Overview

This document describes the testing strategy, test structure, and how to run tests for the Kubernetes State Monitor Operator.

## Test Strategy

The operator implements a comprehensive testing approach with three levels of testing:

1. **Unit Tests**: Test individual components in isolation
2. **Integration Tests**: Test component interactions with mocked Kubernetes API
3. **End-to-End (E2E) Tests**: Test the complete operator in a real Kubernetes cluster

## Test Structure

```
test/
├── e2e/
│   └── statemonitor_test.go    # End-to-end tests
└── utils/
    └── test_helpers.go          # Shared test utilities

controllers/
└── statemonitor_controller_test.go  # Controller unit tests

api/v1alpha1/
└── statemonitor_types_test.go      # API types unit tests
```

## Unit Tests

### Controller Tests (`controllers/statemonitor_controller_test.go`)

Tests the reconciliation logic with various scenarios:

#### Test Cases:
1. **Node to Pod Ratio Calculations**:
   - 1-15 nodes → 1 pod
   - 16-30 nodes → 2 pods
   - 31-45 nodes → 3 pods
   - Custom ratios (e.g., 1:5)

2. **Scaling Behaviors**:
   - Scale up when nodes increase
   - Scale down when nodes decrease
   - Handle zero nodes gracefully

3. **Resource Management**:
   - Apply resource requests and limits
   - Handle partial resource specifications
   - Default values when not specified

4. **Error Handling**:
   - Missing StateMonitor resource
   - Invalid configurations

#### Running Controller Tests:
```bash
go test ./controllers/... -v
```

### API Types Tests (`api/v1alpha1/statemonitor_types_test.go`)

Tests the CRD types and validation:

#### Test Cases:
1. **Type Creation**:
   - Default values initialization
   - Custom values assignment
   - Status field updates

2. **Resource Requirements**:
   - Empty resource specifications
   - Partial resource specifications
   - Complete resource specifications

3. **List Management**:
   - StateMonitorList operations
   - Multiple StateMonitor resources

#### Running API Tests:
```bash
go test ./api/v1alpha1/... -v
```

## Integration Tests

Integration tests use the Kubernetes fake client to simulate cluster operations without requiring a real cluster.

### Key Features:
- Uses `sigs.k8s.io/controller-runtime/pkg/client/fake`
- Tests complete reconciliation cycles
- Validates status updates
- Verifies pod creation/deletion

### Test Scenarios:
1. **Multi-StateMonitor Management**: Multiple CRs managing different pod sets
2. **Concurrent Operations**: Parallel reconciliation of resources
3. **Resource Ownership**: Controller references and garbage collection

## End-to-End Tests (`test/e2e/statemonitor_test.go`)

E2E tests validate the operator in a real Kubernetes environment.

### Prerequisites:
1. Running Kubernetes cluster (kind, minikube, or real cluster)
2. Operator deployed to the cluster
3. Monitoring namespace created

### Test Cases:
1. **Complete Lifecycle Test**:
   - Create StateMonitor
   - Verify pod creation
   - Update node count simulation
   - Verify scaling
   - Delete StateMonitor
   - Verify cleanup

2. **Multiple Resources Test**:
   - Deploy multiple StateMonitors
   - Verify independent management
   - Test resource isolation

### Running E2E Tests:
```bash
# Deploy the operator first
make deploy

# Run E2E tests
go test ./test/e2e/... -v -tags=e2e

# Or use make target
make test-e2e
```

## Test Utilities (`test/utils/test_helpers.go`)

Provides reusable test helpers:

### Helper Functions:
- `CreateTestNodes()`: Generate mock node objects
- `CreateTestStateMonitor()`: Create test CR instances
- `WaitForStateMonitorReady()`: Wait for CR to be ready
- `GetMonitorPods()`: Retrieve pods for a StateMonitor
- `CalculateExpectedReplicas()`: Validate replica calculations
- `AssertPodsHaveCorrectLabels()`: Verify pod labeling

## Running All Tests

### Quick Test Run:
```bash
# Run all unit tests
make test

# Run with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Comprehensive Testing:
```bash
# 1. Format and vet code
make fmt
make vet

# 2. Run unit tests
go test ./api/... ./controllers/... -v

# 3. Run integration tests
go test ./controllers/... -v -run Integration

# 4. Deploy to test cluster
make deploy

# 5. Run E2E tests
go test ./test/e2e/... -v -tags=e2e

# 6. Cleanup
make undeploy
```

## Test Coverage

### Target Coverage Goals:
- Controller logic: >80%
- API types: >90%
- Critical paths: 100%

### Generate Coverage Report:
```bash
go test ./... -coverprofile=cover.out
go tool cover -func=cover.out
go tool cover -html=cover.out -o coverage.html
```

## Continuous Integration

### GitHub Actions Workflow Example:
```yaml
name: Test
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    - name: Run tests
      run: |
        make test
        make vet
    - name: Upload coverage
      uses: codecov/codecov-action@v3
      with:
        files: ./cover.out
```

## Testing Best Practices

### 1. Use Ginkgo and Gomega
- BDD-style test descriptions
- Rich assertion matchers
- Better test organization

### 2. Table-Driven Tests
```go
testCases := []struct {
    name      string
    nodeCount int
    expected  int
}{
    {"small cluster", 10, 1},
    {"medium cluster", 30, 2},
    {"large cluster", 100, 7},
}
```

### 3. Mock External Dependencies
- Use fake clients for Kubernetes API
- Mock time for time-based operations
- Isolate network calls

### 4. Test Data Management
- Use test utilities for consistent test data
- Clean up resources after tests
- Avoid hardcoded values

### 5. Parallel Testing
```go
It("should handle concurrent updates", func() {
    // Use goroutines for parallel operations
    // Verify thread safety
})
```

## Debugging Tests

### Verbose Output:
```bash
go test -v ./controllers/...
```

### Focus on Specific Tests:
```go
FIt("should focus on this test", func() {
    // Only this test runs
})
```

### Skip Tests:
```go
XIt("should skip this test", func() {
    // This test is skipped
})
```

### Debug with Delve:
```bash
dlv test ./controllers -- -test.run TestStateMonitorController
```

## Performance Testing

### Benchmarks:
```go
func BenchmarkReconcile(b *testing.B) {
    for i := 0; i < b.N; i++ {
        reconciler.Reconcile(ctx, req)
    }
}
```

### Run Benchmarks:
```bash
go test -bench=. -benchmem ./controllers/...
```

## Test Maintenance

### Regular Tasks:
1. Update test dependencies monthly
2. Review and update test coverage quarterly
3. Refactor tests when implementation changes
4. Add tests for bug fixes
5. Document complex test scenarios

### Test Review Checklist:
- [ ] Tests cover happy path
- [ ] Tests cover error conditions
- [ ] Tests are deterministic
- [ ] Tests clean up resources
- [ ] Tests run in reasonable time
- [ ] Tests have clear descriptions

## Troubleshooting

### Common Issues:

1. **Tests fail with "connection refused"**:
   - Ensure Kubernetes cluster is running
   - Check KUBECONFIG is set correctly

2. **E2E tests timeout**:
   - Increase timeout values
   - Check operator logs
   - Verify cluster resources

3. **Flaky tests**:
   - Add proper wait conditions
   - Use Eventually() for async operations
   - Avoid time.Sleep()

4. **Resource conflicts**:
   - Use unique names for test resources
   - Implement proper cleanup
   - Run tests with -parallel=1 if needed

## Additional Resources

- [Ginkgo Documentation](https://onsi.github.io/ginkgo/)
- [Gomega Matchers](https://onsi.github.io/gomega/)
- [Controller Runtime Testing](https://book.kubebuilder.io/reference/testing.html)
- [Kubernetes Testing SIG](https://github.com/kubernetes/community/tree/master/sig-testing)