.PHONY: install update-deps generate patch dev database-delete app-data-delete storage-delete

install:
	cd app && bun install

update-deps:
	cd code-gen && npx buf dep update

patch:
	go mod vendor
	cd vendor/github.com/AlexxIT/go2rtc && patch -p1 < ../../../../patches/go2rtc-setconn.patch
	cd ../../../..

generate: generate-ts generate-go

generate-ts:
	rm -rf app/gen/unblink
	cd app && npx buf generate ../code-gen --template ../code-gen/buf.gen.ts.yaml

generate-go:
	rm -rf code-gen/gen/unblink
	cd relay && npx buf generate ../code-gen --template ../code-gen/buf.gen.go.yaml

database-delete:
	cd relay && go run ../cmd/relay/ database delete

app-data-delete:
	cd relay && go run ../cmd/relay/ app-data delete

storage-delete:
	cd relay && go run ../cmd/relay/ storage delete