// SPDX-License-Identifier: MPL-2.0

package blob_test

import (
	"fmt"

	"github.com/oioio-space/inkwell/protocol/blob"
)

func ExampleDecode() {
	b, err := blob.Encode(blob.Want{Hash: blob.HashOf([]byte("hello")), Length: blob.GroupSize})
	if err != nil {
		panic(err)
	}
	m, err := blob.Decode(b)
	if err != nil {
		panic(err)
	}
	fmt.Println(m.(blob.Want).Length)
	// Output: 16384
}
