// Separate module so the parity benchmarks are excluded from the parent
// module's `go test ./...` / 100% coverage gate. Run with:
//
//	cd benchmarks && go test -bench=. -benchmem -cpu=1 ./...
module github.com/go-images/images/benchmarks

go 1.27.1

require github.com/go-images/images v0.1.0

require (
	github.com/ajroetker/go-highway v0.0.12 // indirect
	github.com/go-gfx/gfx v0.35.0 // indirect
	github.com/go-images/gif v0.2.0 // indirect
	github.com/go-images/jpeg v0.3.0 // indirect
	github.com/go-images/jpeg2000 v0.13.3 // indirect
	github.com/go-images/png v0.2.0 // indirect
	github.com/sergeymakinen/go-bmp v1.0.0 // indirect
	github.com/sergeymakinen/go-ico v1.0.0 // indirect
	github.com/tannevaled/gobig2 v0.2.0 // indirect
	golang.org/x/image v0.47.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
)

replace github.com/go-images/images => ../
