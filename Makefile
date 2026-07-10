.PHONY: run build keys clean

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

keys:
	mkdir -p keys
	openssl genpkey -algorithm RSA -out keys/private.pem -pkeyopt rsa_keygen_bits:2048
	openssl rsa -pubout -in keys/private.pem -out keys/public.pem

clean:
	rm -rf bin/ tmp/
