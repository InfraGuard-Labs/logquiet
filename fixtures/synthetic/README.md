# Synthetic fixtures

**Every file in this directory is synthetic, fabricated test data.** None of
it is derived from any real system, company, incident, or user. Hostnames,
IPs, usernames, and timestamps are all generated. Files are produced by
[`tools/fixturegen`](../../tools/fixturegen/main.go) so they are exactly
reproducible: `go run ./tools/fixturegen correctness`.

Each `<scenario>.log` has a matching `<scenario>.manifest.json` listing the
substrings that a correct implementation must never suppress ("known
important" events), used by the correctness benchmark harness in
[`benchmarks/`](../../benchmarks/). See [docs/BENCHMARKS.md](../../docs/BENCHMARKS.md).

| Fixture | Scenario |
|---|---|
| `k8s-pod-restart-loop.log` | Kubernetes pod stuck in CrashLoopBackOff |
| `database-connection-exhaustion.log` | Pool saturates, times out, recovers |
| `http-service-failure.log` | Healthy 200s degrade into 503s and an upstream failure |
| `java-spring-exception.log` | NullPointerException with a chained cause |
| `python-traceback.log` | Unhandled ZeroDivisionError in a worker |
| `nodejs-exception.log` | Unhandled promise rejection in an Express app |
| `go-panic.log` | Index-out-of-range panic with goroutine trace |
| `ci-deployment-failure.log` | CI pipeline succeeds until the final registry push fails |
| `auth-failure-burst.log` | Brute-force-style authentication failure burst and lockout |
| `healthy-then-fatal.log` | 2000 lines of diverse routine chatter ending in an OOM kill |
| `spec-example.log` | The worked example from the original product specification |
