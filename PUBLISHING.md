# Publishing

Go modules are published by pushing a semver tag; there is no registry upload.

1. Update `Version` in `client.go` and add a CHANGELOG entry.
2. Make sure CI is green on `main`, then tag and push:

   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```

3. Ask the module proxy to index the release so it shows up on pkg.go.dev right away:

   ```bash
   GOPROXY=https://proxy.golang.org go list -m github.com/mista-io/mista-go@v0.1.0
   ```

Users then install it with `go get github.com/mista-io/mista-go@v0.1.0`.
