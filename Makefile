.PHONY: run build keys clean bench loadtest loadtest-build integration

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

keys:
	mkdir -p keys
	openssl genpkey -algorithm RSA -out keys/private.pem -pkeyopt rsa_keygen_bits:2048
	openssl rsa -pubout -in keys/private.pem -out keys/public.pem

# --- Benchmarks ---
bench:
	go test -bench=. -benchmem ./tests/bench/

bench-cpu:
	go test -bench=. -benchmem -cpuprofile=cpu.prof -memprofile=mem.prof ./tests/bench/

# --- Load test (standalone binary) ---
loadtest-build:
	go build -o bin/loadtest ./cmd/loadtest

loadtest: loadtest-build
	bin/loadtest -url=http://localhost:8080 -c=20 -d=30s -warmup=5s

loadtest-heavy: loadtest-build
	bin/loadtest -url=http://localhost:8080 -c=100 -d=60s -warmup=10s

loadtest-max: loadtest-build
	bin/loadtest -url=http://localhost:8080 -c=500 -d=60s -warmup=15s -o=results.json

# --- Integration tests (requires DB_HOST env) ---
integration:
	go test -tags=integration -v -count=1 ./tests/integration/

integration-short:
	go test -tags=integration -v -count=1 -run 'TestFullFlow|TestAdminFlow' ./tests/integration/

# --- k6 load test ---
k6-load:
	k6 run load-test.js -e BASE_URL=http://localhost:8080 -e TARGET_RPS=50

k6-load-heavy:
	k6 run load-test.js -e BASE_URL=http://localhost:8080 -e TARGET_RPS=200

k6-spike:
	k6 run load-test.js -e BASE_URL=http://localhost:8080 -e TARGET_RPS=500 -e ADMIN_EMAIL=admin@example.com -e ADMIN_PASSWORD=admin123!

# --- Go tools ---
vet:
	go vet ./...

clean:
	rm -rf bin/ tmp/
