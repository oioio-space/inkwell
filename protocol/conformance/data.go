// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"math/big"
	"strings"

	"github.com/oioio-space/inkwell/protocol/vectors"
)

func vectorsData(name string) ([]byte, error) { return vectors.FS.ReadFile(name) }
func base58(s string) []byte {
	n := new(big.Int)
	for _, r := range s {
		i := strings.IndexRune("123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz", r)
		if i < 0 {
			return nil
		}
		n.Mul(n, big.NewInt(58))
		n.Add(n, big.NewInt(int64(i)))
	}
	b := n.Bytes()
	for _, r := range s {
		if r != '1' {
			break
		}
		b = append([]byte{0}, b...)
	}
	return b
}
