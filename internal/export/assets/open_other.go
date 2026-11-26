//go:build !unix

package assets

import "os"

func openRegularCandidate(root *os.Root, path string) (*os.File, error) {
	return root.Open(path)
}
