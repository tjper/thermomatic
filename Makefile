.PHONY: help
help: ## show this help
			@fgrep -h "##" $(MAKEFILE_LIST) | fgrep -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'

.PHONY: test
test: ## execute all of thermomatic's tests
	@go test -v -race -count=1 -tags=integration ./...

.PHONY: unit-test
unit-test: ## execute all of thermomatic's unit tests
	@go test -v -race -count=1 ./...

.PHONY: benchmark
benchmark: ## execute all of thermomatic's benchmarks
	@go test -v -count=10 -run XXX -bench . -benchmem ./...
