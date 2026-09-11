package main

import (
	_ "embed"
	"encoding/json"
	"log"
)

// productJSON is projected out of build/config.yml by
// `task common:generate:product`, and committed so a plain `go build` works
// without running the task first.
//
// The values cannot travel as -ldflags: `go build` splits the -ldflags value on
// spaces and ignores quoting, so a name like "JSON Inspector" is truncated at
// the space and the link fails. One generated file, read by both Go and the
// frontend, keeps the metadata single-sourced instead.
//
//go:embed frontend/src/product.json
var productJSON []byte

// productMetadata mirrors frontend/src/product.json.
type productMetadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// product is read once at start-up. A malformed file means the generator and
// this struct have drifted apart, which is worth failing loudly for.
var product = func() productMetadata {
	var p productMetadata
	if err := json.Unmarshal(productJSON, &p); err != nil {
		log.Fatalf("product.json is not valid JSON: %v", err)
	}
	if p.Name == "" {
		log.Fatal("product.json has no name — run `task common:generate:product`")
	}
	return p
}()
