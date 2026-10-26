package typst

import (
	"bytes"
	"errors"
	"strconv"
)

// PageCount returns the number of pages of a PDF made by Typst, from the page
// tree at its root. It is how the preview knows the length of the document
// while rendering only the pages in view. It reads what Typst writes (plain
// objects, no object streams) and refuses anything else.
func PageCount(pdf []byte) (int, error) {
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		return 0, errors.New("typst: not a PDF")
	}
	objs, order, err := readObjects(pdf)
	if err != nil {
		return 0, err
	}
	for _, num := range order {
		d := objs[num].dict
		if d["Type"] != "/Pages" {
			continue
		}
		if _, nested := d["Parent"]; nested {
			continue // the root of the tree is the node without a parent
		}
		n, err := strconv.Atoi(d["Count"])
		if err != nil || n < 0 || n > DefaultMaxPages*100 {
			return 0, errors.New("typst: the page tree has no valid /Count")
		}
		return n, nil
	}
	return 0, errors.New("typst: no page tree in the PDF")
}
