//go:build tools

// Pins lint-only dependencies so `go mod tidy` keeps them; never compiled.
package tools

import _ "github.com/quasilyte/go-ruleguard/dsl"
