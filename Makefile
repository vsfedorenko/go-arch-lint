tests:
	go test ./...

tests-functional:
	go test ./test/check/...

arch-next:
	@echo "-version:"
	go run ./cmd/arch-lint/ version
	@echo "-status:"
	go run ./cmd/arch-lint/ check

arch-prev:
	@echo "-version:"
	docker run --rm vsfedorenko/go-arch-lint:latest-stable-release version
	@echo "-status:"
	docker run --rm \
		-v ${PWD}:/app \
		vsfedorenko/go-arch-lint:latest-stable-release check --project-path /app

release-dry:
	@echo "check config.."
	goreleaser check
	@echo "build dry release.."
	goreleaser --snapshot --skip-publish --rm-dist
