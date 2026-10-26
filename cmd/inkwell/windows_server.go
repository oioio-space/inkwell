//go:build server

package main

// newWindowHost returns nothing in the server build, which has no native window:
// the page opens the public view in a browser tab itself.
func newWindowHost() windowHost { return nil }
