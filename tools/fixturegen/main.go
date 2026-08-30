// Command fixturegen produces the synthetic fixtures used by LogQuiet's
// correctness benchmark suite (fixtures/synthetic/*.log) and raw
// performance-benchmark corpora (benchmarks/data/*.log). All output is
// synthetic/fabricated data generated from templates below - none of it is
// derived from any real system, user, or incident.
//
// Usage:
//
//	go run ./tools/fixturegen correctness    # (re)writes fixtures/synthetic/*
//	go run ./tools/fixturegen perf 1000000 mixed benchmarks/data/mixed-1m.log
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: fixturegen correctness | fixturegen perf <lines> <profile> <outfile>")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "correctness":
		genCorrectness()
	case "perf":
		if len(os.Args) != 5 {
			fmt.Fprintln(os.Stderr, "usage: fixturegen perf <lines> <repetitive|diverse|mixed> <outfile>")
			os.Exit(2)
		}
		var n int
		fmt.Sscanf(os.Args[2], "%d", &n)
		genPerf(n, os.Args[3], os.Args[4])
	default:
		fmt.Fprintln(os.Stderr, "unknown subcommand:", os.Args[1])
		os.Exit(2)
	}
}

type manifest struct {
	Scenario       string   `json:"scenario"`
	Synthetic      bool     `json:"synthetic"`
	TotalLines     int      `json:"total_lines"`
	KnownImportant []string `json:"known_important_substrings"`
	Description    string   `json:"description"`
}

func writeLines(path string, lines []string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		panic(err)
	}
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	w := bufio.NewWriterSize(f, 1<<20)
	defer w.Flush()
	for _, l := range lines {
		w.WriteString(l)
		w.WriteByte('\n')
	}
}

func writeManifest(path string, m manifest) {
	b, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(path, b, 0o644)
}

func ts(sec int) string {
	h := 3 + sec/3600
	m := (sec / 60) % 60
	s := sec % 60
	return fmt.Sprintf("2026-08-30 %02d:%02d:%02d", h, m, s)
}

// ---- correctness scenarios -------------------------------------------------

func genCorrectness() {
	dir := "fixtures/synthetic"

	k8sRestartLoop(dir)
	dbExhaustion(dir)
	httpFailure(dir)
	javaException(dir)
	pythonTraceback(dir)
	nodeException(dir)
	goPanicScenario(dir)
	ciFailure(dir)
	authBurst(dir)
	healthyThenFatal(dir)
}

func k8sRestartLoop(dir string) {
	var lines []string
	sec := 0
	for cycle := 0; cycle < 40; cycle++ {
		lines = append(lines, fmt.Sprintf(`%s 1 pod/payment-api-7c9f6d8b4d-x2k9p Normal Pulling Pulling image "registry.internal/payment-api:v1.4.2"`, ts(sec)))
		sec++
		lines = append(lines, fmt.Sprintf(`%s 1 pod/payment-api-7c9f6d8b4d-x2k9p Normal Pulled Successfully pulled image "registry.internal/payment-api:v1.4.2" in 812ms`, ts(sec)))
		sec++
		lines = append(lines, fmt.Sprintf(`%s 1 pod/payment-api-7c9f6d8b4d-x2k9p Normal Created Created container payment-api`, ts(sec)))
		sec++
		lines = append(lines, fmt.Sprintf(`%s 1 pod/payment-api-7c9f6d8b4d-x2k9p Normal Started Started container payment-api`, ts(sec)))
		sec++
		lines = append(lines, fmt.Sprintf(`%s [INFO] payment-api listening on :8080`, ts(sec)))
		sec++
		lines = append(lines, fmt.Sprintf(`%s [INFO] payment-api health check ok`, ts(sec)))
		sec++
		lines = append(lines, fmt.Sprintf(`%s [FATAL] panic: failed to connect to config store: dial tcp 10.4.0.9:2379: connect: connection refused`, ts(sec)))
		sec++
		lines = append(lines, fmt.Sprintf(`%s 1 pod/payment-api-7c9f6d8b4d-x2k9p Warning BackOff Back-off restarting failed container`, ts(sec)))
		sec++
	}
	lines = append(lines, fmt.Sprintf(`%s 1 pod/payment-api-7c9f6d8b4d-x2k9p Warning Unhealthy Readiness probe failed: dial tcp 10.4.2.11:8080: connect: connection refused (CrashLoopBackOff after 40 restarts)`, ts(sec)))

	writeLines(filepath.Join(dir, "k8s-pod-restart-loop.log"), lines)
	writeManifest(filepath.Join(dir, "k8s-pod-restart-loop.manifest.json"), manifest{
		Scenario: "kubernetes-pod-restart-loop", Synthetic: true, TotalLines: len(lines),
		KnownImportant: []string{
			"panic: failed to connect to config store",
			"CrashLoopBackOff after 40 restarts",
		},
		Description: "40 restart cycles of a pod that repeatedly panics connecting to its config store, ending in a CrashLoopBackOff readiness failure.",
	})
}

func dbExhaustion(dir string) {
	var lines []string
	sec := 0
	for i := 0; i < 300; i++ {
		lines = append(lines, fmt.Sprintf(`%s [INFO] Connection pool active. %d connections open.`, ts(sec), 42))
		sec++
	}
	for i := 0; i < 25; i++ {
		lines = append(lines, fmt.Sprintf(`%s [WARNING] Connection pool at capacity: 50/50 connections in use`, ts(sec)))
		sec++
	}
	lines = append(lines, fmt.Sprintf(`%s [CRITICAL] DATABASE TIMEOUT: Host 10.0.1.45 failed to respond in 5000ms.`, ts(sec)))
	sec++
	lines = append(lines, fmt.Sprintf(`%s [ERROR] Traceback (most recent call last):`, ts(sec)))
	lines = append(lines, fmt.Sprintf(`%s [ERROR]   File "/app/db.py", line 42, in execute_query`, ts(sec)))
	lines = append(lines, fmt.Sprintf(`%s [ERROR]     raise TimeoutError("DB connection lost")`, ts(sec)))
	lines = append(lines, fmt.Sprintf(`%s [ERROR] TimeoutError: DB connection lost`, ts(sec)))
	sec++
	for i := 1; i <= 6; i++ {
		lines = append(lines, fmt.Sprintf(`%s [WARNING] Retrying connection attempt %d...`, ts(sec), i))
		sec++
	}
	lines = append(lines, fmt.Sprintf(`%s [INFO] Connection pool recovered. 12 connections open.`, ts(sec)))

	writeLines(filepath.Join(dir, "database-connection-exhaustion.log"), lines)
	writeManifest(filepath.Join(dir, "database-connection-exhaustion.manifest.json"), manifest{
		Scenario: "database-connection-exhaustion", Synthetic: true, TotalLines: len(lines),
		KnownImportant: []string{
			"DATABASE TIMEOUT: Host 10.0.1.45",
			"TimeoutError: DB connection lost",
		},
		Description: "A healthy connection pool gradually saturates, times out, and recovers after retries.",
	})
}

func httpFailure(dir string) {
	var lines []string
	sec := 0
	paths := []string{"/api/v1/orders", "/api/v1/users", "/api/v1/cart", "/healthz"}
	for i := 0; i < 400; i++ {
		lines = append(lines, fmt.Sprintf(`10.0.%d.%d - - [30/Aug/2026:03:%02d:%02d +0000] "GET %s HTTP/1.1" 200 512 0.014`,
			i%256, (i*7)%256, (sec/60)%60, sec%60, paths[i%len(paths)]))
		sec++
	}
	for i := 0; i < 120; i++ {
		lines = append(lines, fmt.Sprintf(`10.0.%d.%d - - [30/Aug/2026:03:%02d:%02d +0000] "GET /api/v1/orders HTTP/1.1" 503 89 4.982`,
			i%256, (i*3)%256, (sec/60)%60, sec%60))
		sec++
	}
	lines = append(lines, fmt.Sprintf(`%s [CRITICAL] upstream connect error or disconnect/reset before headers. reset reason: connection failure, transport failure reason: delayed connect error: 111`, ts(sec)))

	writeLines(filepath.Join(dir, "http-service-failure.log"), lines)
	writeManifest(filepath.Join(dir, "http-service-failure.manifest.json"), manifest{
		Scenario: "http-service-failure", Synthetic: true, TotalLines: len(lines),
		KnownImportant: []string{
			"upstream connect error or disconnect/reset before headers",
		},
		Description: "Nginx/Envoy-style access log: healthy 200s, a burst of 503s, then an explicit upstream connect failure.",
	})
}

func javaException(dir string) {
	var lines []string
	sec := 0
	for i := 0; i < 200; i++ {
		lines = append(lines, fmt.Sprintf(`%s INFO  [http-nio-8080-exec-%d] c.e.OrderController - Processed order %d in 23ms`, ts(sec), i%20+1, 100000+i))
		sec++
	}
	lines = append(lines, fmt.Sprintf(`%s ERROR [http-nio-8080-exec-7] c.e.OrderController - java.lang.NullPointerException: Cannot invoke "com.example.Customer.getId()" because "customer" is null`, ts(sec)))
	lines = append(lines, `	at com.example.OrderService.charge(OrderService.java:88)`)
	lines = append(lines, `	at com.example.OrderController.create(OrderController.java:41)`)
	lines = append(lines, `	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke0(Native Method)`)
	lines = append(lines, `Caused by: java.lang.IllegalStateException: customer session expired`)
	lines = append(lines, `	at com.example.SessionManager.get(SessionManager.java:15)`)
	lines = append(lines, `	... 5 more`)
	sec++
	for i := 0; i < 30; i++ {
		lines = append(lines, fmt.Sprintf(`%s INFO  [http-nio-8080-exec-%d] c.e.OrderController - Processed order %d in 19ms`, ts(sec), i%20+1, 200000+i))
		sec++
	}

	writeLines(filepath.Join(dir, "java-spring-exception.log"), lines)
	writeManifest(filepath.Join(dir, "java-spring-exception.manifest.json"), manifest{
		Scenario: "java-exception", Synthetic: true, TotalLines: len(lines),
		KnownImportant: []string{
			"java.lang.NullPointerException",
			"Caused by: java.lang.IllegalStateException",
		},
		Description: "A Spring Boot access-log stream with one NullPointerException with a chained cause in the middle.",
	})
}

func pythonTraceback(dir string) {
	var lines []string
	sec := 0
	for i := 0; i < 150; i++ {
		lines = append(lines, fmt.Sprintf(`%s [INFO] worker received task task-%d`, ts(sec), 5000+i))
		sec++
	}
	lines = append(lines, fmt.Sprintf(`%s [ERROR] Traceback (most recent call last):`, ts(sec)))
	lines = append(lines, `  File "/app/worker.py", line 88, in process`)
	lines = append(lines, `    result = transform(payload)`)
	lines = append(lines, `  File "/app/transform.py", line 12, in transform`)
	lines = append(lines, `    return payload["amount"] / payload["rate"]`)
	lines = append(lines, `ZeroDivisionError: division by zero`)
	sec++
	for i := 0; i < 40; i++ {
		lines = append(lines, fmt.Sprintf(`%s [INFO] worker received task task-%d`, ts(sec), 6000+i))
		sec++
	}

	writeLines(filepath.Join(dir, "python-traceback.log"), lines)
	writeManifest(filepath.Join(dir, "python-traceback.manifest.json"), manifest{
		Scenario: "python-traceback", Synthetic: true, TotalLines: len(lines),
		KnownImportant: []string{"ZeroDivisionError: division by zero"},
		Description:    "A Celery-style worker log with one unhandled ZeroDivisionError traceback amid routine task processing.",
	})
}

func nodeException(dir string) {
	var lines []string
	sec := 0
	for i := 0; i < 150; i++ {
		lines = append(lines, fmt.Sprintf(`%s info: request completed method=GET path=/api/items status=200 duration=%dms`, ts(sec), 10+i%40))
		sec++
	}
	lines = append(lines, fmt.Sprintf(`%s error: Unhandled promise rejection: TypeError: Cannot read properties of undefined (reading 'id')`, ts(sec)))
	lines = append(lines, `    at ItemService.get (/app/src/services/item.js:34:17)`)
	lines = append(lines, `    at async ItemController.show (/app/src/controllers/item.js:12:20)`)
	lines = append(lines, `    at async Layer.handle [as handle_request] (/app/node_modules/express/lib/router/layer.js:95:5)`)
	sec++
	for i := 0; i < 40; i++ {
		lines = append(lines, fmt.Sprintf(`%s info: request completed method=GET path=/api/items status=200 duration=%dms`, ts(sec), 12+i%30))
		sec++
	}

	writeLines(filepath.Join(dir, "nodejs-exception.log"), lines)
	writeManifest(filepath.Join(dir, "nodejs-exception.manifest.json"), manifest{
		Scenario: "nodejs-exception", Synthetic: true, TotalLines: len(lines),
		KnownImportant: []string{"Unhandled promise rejection: TypeError"},
		Description:    "An Express request log with one unhandled promise rejection stack trace.",
	})
}

func goPanicScenario(dir string) {
	var lines []string
	sec := 0
	for i := 0; i < 100; i++ {
		lines = append(lines, fmt.Sprintf(`%s level=info msg="request handled" path=/v1/quote status=200 dur=%dms`, ts(sec), 3+i%9))
		sec++
	}
	lines = append(lines, `panic: runtime error: index out of range [5] with length 3`)
	lines = append(lines, ``)
	lines = append(lines, `goroutine 1 [running]:`)
	lines = append(lines, `main.computeQuote(...)`)
	lines = append(lines, `	/app/quote.go:42 +0x1b`)
	lines = append(lines, `main.main()`)
	lines = append(lines, `	/app/main.go:10 +0x65`)
	lines = append(lines, `exit status 2`)
	sec++
	for i := 0; i < 30; i++ {
		lines = append(lines, fmt.Sprintf(`%s level=info msg="request handled" path=/v1/quote status=200 dur=%dms`, ts(sec), 4+i%9))
		sec++
	}

	writeLines(filepath.Join(dir, "go-panic.log"), lines)
	writeManifest(filepath.Join(dir, "go-panic.manifest.json"), manifest{
		Scenario: "go-panic", Synthetic: true, TotalLines: len(lines),
		// Note: LogQuiet's renderer consumes the literal word "panic:" as a
		// FATAL severity token (replacing it with its own icon+label), so
		// the checked substring is the diagnostic payload that must
		// survive, not the literal Go runtime prefix formatting.
		KnownImportant: []string{"runtime error: index out of range"},
		Description:    "A Go service log with one index-out-of-range panic and full goroutine trace.",
	})
}

func ciFailure(dir string) {
	var lines []string
	sec := 0
	steps := []string{"checkout", "install-deps", "lint", "unit-tests", "build", "integration-tests", "package", "deploy"}
	for _, s := range steps[:6] {
		lines = append(lines, fmt.Sprintf(`%s ##[group]Run step: %s`, ts(sec), s))
		sec++
		for i := 0; i < 15; i++ {
			lines = append(lines, fmt.Sprintf(`%s [%s] ok - step %d/15`, ts(sec), s, i+1))
			sec++
		}
		lines = append(lines, fmt.Sprintf(`%s ##[endgroup]`, ts(sec)))
		sec++
	}
	lines = append(lines, fmt.Sprintf(`%s ##[group]Run step: package`, ts(sec)))
	sec++
	lines = append(lines, fmt.Sprintf(`%s [package] ERROR: failed to push image registry.internal/payment-api:v1.4.3: unauthorized: authentication required`, ts(sec)))
	sec++
	lines = append(lines, fmt.Sprintf(`%s ##[error]Process completed with exit code 1.`, ts(sec)))

	writeLines(filepath.Join(dir, "ci-deployment-failure.log"), lines)
	writeManifest(filepath.Join(dir, "ci-deployment-failure.manifest.json"), manifest{
		Scenario: "ci-deployment-failure", Synthetic: true, TotalLines: len(lines),
		KnownImportant: []string{
			"failed to push image",
			"Process completed with exit code 1",
		},
		Description: "A GitHub-Actions-style CI log where every step succeeds until the final registry push fails on auth.",
	})
}

func authBurst(dir string) {
	var lines []string
	sec := 0
	for i := 0; i < 50; i++ {
		lines = append(lines, fmt.Sprintf(`%s [INFO] user admin@example.com logged in from 203.0.113.%d`, ts(sec), i%50))
		sec++
	}
	for i := 0; i < 500; i++ {
		// All within the same wall-clock second: a rapid brute-force burst.
		lines = append(lines, fmt.Sprintf(`%s [WARN] authentication failed for user admin from 198.51.100.%d: invalid password`, ts(sec), i%256))
	}
	lines = append(lines, fmt.Sprintf(`%s [CRITICAL] account admin locked after 500 failed login attempts in under 60 seconds - possible brute force attack`, ts(sec+1)))

	writeLines(filepath.Join(dir, "auth-failure-burst.log"), lines)
	writeManifest(filepath.Join(dir, "auth-failure-burst.manifest.json"), manifest{
		Scenario: "authentication-failure-burst", Synthetic: true, TotalLines: len(lines),
		KnownImportant: []string{
			"account admin locked after 500 failed login attempts",
		},
		Description: "Normal logins followed by a rapid brute-force-style authentication failure burst and an account lockout.",
	})
}

func healthyThenFatal(dir string) {
	var lines []string
	sec := 0
	rng := rand.New(rand.NewSource(42))
	msgs := []string{
		"cache hit for key session:%d",
		"cache miss for key session:%d, fetched from origin",
		"scheduled job heartbeat tick %d",
		"metrics flushed: %d series",
	}
	for i := 0; i < 2000; i++ {
		m := msgs[rng.Intn(len(msgs))]
		lines = append(lines, fmt.Sprintf(`%s [INFO] `+m, ts(sec), i))
		sec++
	}
	lines = append(lines, fmt.Sprintf(`%s [FATAL] out of memory: cannot allocate 512MB, killing process`, ts(sec)))

	writeLines(filepath.Join(dir, "healthy-then-fatal.log"), lines)
	writeManifest(filepath.Join(dir, "healthy-then-fatal.manifest.json"), manifest{
		Scenario: "healthy-noisy-service-then-fatal", Synthetic: true, TotalLines: len(lines),
		KnownImportant: []string{"out of memory: cannot allocate 512MB"},
		Description:    "2000 lines of routine, moderately diverse background chatter ending in a single fatal OOM kill.",
	})
}

// ---- raw performance corpora -----------------------------------------------

func genPerf(n int, profile, outfile string) {
	rng := rand.New(rand.NewSource(7))
	f, err := os.Create(outfile)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	w := bufio.NewWriterSize(f, 4<<20)
	defer w.Flush()

	services := []string{"api", "worker", "cache", "gateway", "auth"}
	verbs := []string{"handled request", "processed job", "flushed batch", "renewed lease", "refreshed token"}

	for i := 0; i < n; i++ {
		sec := i / 37
		var repetitive bool
		switch profile {
		case "repetitive":
			repetitive = true
		case "diverse":
			repetitive = false
		default: // mixed
			repetitive = rng.Intn(100) < 90
		}
		if repetitive {
			fmt.Fprintf(w, "%s [INFO] %s: connection pool active, %d connections open\n", ts(sec), services[i%len(services)], 40+i%5)
		} else {
			fmt.Fprintf(w, "%s [INFO] %s: %s id=%d dur=%dms addr=10.%d.%d.%d\n",
				ts(sec), services[i%len(services)], verbs[i%len(verbs)], rng.Intn(1_000_000), rng.Intn(500),
				rng.Intn(256), rng.Intn(256), rng.Intn(256))
		}
		if i%50000 == 49999 {
			fmt.Fprintf(w, "%s [ERROR] transient failure talking to dependency %s\n", ts(sec), services[i%len(services)])
		}
	}
}
