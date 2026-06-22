// Separate module so the parity benchmarks are excluded from the parent
// module's `go test ./...` / 100% coverage gate. Run with:
//
//	cd benchmarks && go test -bench=. -benchmem -cpu=1 ./...
module github.com/go-images/images/benchmarks

go 1.26.4

require github.com/go-images/images v0.0.0

replace github.com/go-images/images => ../
