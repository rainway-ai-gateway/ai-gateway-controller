# /run-int-tests — ai-gateway-controller 集成测试飞轮

Run the mock-based integration tests (`test/integration/`) and auto-fix failures in a closed loop. No real k8s is used — k8s data (Service/Endpoints/ConfigMap/Pod/InferencePool) is mocked via controller-runtime fake client, and the ai-gateway-api is mocked via httptest.

## Usage
```
/run-int-tests
```

## Execution Steps

### Step 1: Setup Environment
```bash
cd "$(git rev-parse --show-toplevel)"
```

### Step 2: Build & Vet
```bash
go build ./...
go vet ./...
```

### Step 3: Run Integration Tests
```bash
sh test/scripts/run_tests.sh 2>&1 | tee test/integration/artifacts/iter-0.log
```

### Step 4: Core Loop — Auto-Fix (max 10 iterations)
```
iteration = 0
while tests fail AND iteration < 10:
    iteration += 1
    log: test/integration/artifacts/flywheel-iter-<N>.log

    for each failed test:
        read tests/<feature>/design.md + design-docs/sys-design/*.md
        read code file:line
        read assertion: expected vs actual

        Decision tree:
        - actual behavior reasonable, assertion too strict → fix *_test.go, log reason
        - design vs test conflict → PAUSE, ask user
        - code doesn't match design → fix code (minimal), log reason
        - same test fails 2 consecutive iterations → skip test

    After fixes:
        → go vet ./...
        → go build ./...
        → re-run tests (Step 3) into flywheel-iter-<N>.log
```

### Step 5: Report
- All pass: "All integration tests passed after N iterations."
- Limit reached: "STOPPED after 10 iterations. Manual investigation needed."
  - List remaining failures with hints
  - List all changes from flywheel-iter-*.log

## Safety Rules
1. Do not modify code just to pass tests; base changes on logic. If a test is
   unreasonable, fix the test and explain. Always report what/why.
2. NEVER auto-commit or push to git.
3. Only modify: `internal/`, `cmd/`, `test/integration/`, `design-docs/`.
4. Do NOT modify: deploy yaml, Dockerfile, build scripts, go.mod, VERSION file.
5. Design vs test conflict → PAUSE, ask user.
6. Record EVERY change in flywheel-iter-<N>.log with timestamp and reason.

## File Locations
| File | Purpose |
|---|---|
| `test/docs/README.md` | 集成测试方案 |
| `test/integration/testutil/*.go` | 测试工具（ALB mock / reconciler / fake k8s / asserts） |
| `test/integration/tests/<feature>/design.md` | 特性用例设计契约 |
| `test/integration/tests/<feature>/*_test.go` | 可执行用例 |
| `test/integration/artifacts/` | 飞轮日志（gitignore） |
| `test/scripts/run_tests.sh` | 一键运行脚本 |
