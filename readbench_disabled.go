//go:build !readbench

package main

// The native benchmark hook is absent from ordinary application builds.
func configureReadBench(_ *documentHost) {}
