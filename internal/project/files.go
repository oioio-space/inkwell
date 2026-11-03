package project

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Kind classifies a project file for the file list (icon, how to open it).
type Kind uint8

// The kinds. The numeric values are part of the store stream protocol.
const (
	// KindOther is a file Inkwell cannot show as text or as a known format.
	KindOther Kind = iota
	// KindTypst is a Typst source (.typ).
	KindTypst
	// KindMarkdown is a Markdown document (.md, .markdown).
	KindMarkdown
	// KindText is any other text, recognized by extension or by content.
	KindText
	// KindCSV is a table (.csv, .tsv).
	KindCSV
	// KindBib is a bibliography (.bib).
	KindBib
	// KindImage is a picture (.png, .jpg, .gif, .webp, .svg, ...).
	KindImage
	// KindPDF is a PDF document.
	KindPDF
	// KindData is structured data (.json, .yaml, .toml, .xml, ...).
	KindData
)

// String returns the name used in the interface and the tests: "typst",
// "markdown", "text", "csv", "bib", "image", "pdf", "data" or "other".
func (k Kind) String() string {
	switch k {
	case KindTypst:
		return "typst"
	case KindMarkdown:
		return "markdown"
	case KindText:
		return "text"
	case KindCSV:
		return "csv"
	case KindBib:
		return "bib"
	case KindImage:
		return "image"
	case KindPDF:
		return "pdf"
	case KindData:
		return "data"
	}
	return "other"
}

// File is an entry of the project's file list.
type File struct {
	// Path is relative to the project folder, with forward slashes.
	Path string
	// Kind says what the file is.
	Kind Kind
	// Size is its length in bytes.
	Size int64
	// Main is true for the project's main document, [MainFile].
	Main bool
}

const (
	// sniffLen is how much of an unknown file is read to tell text from binary.
	sniffLen = 4096
	// maxListDepth bounds the recursion of [Project.List].
	maxListDepth = 64
	tmpSuffix    = ".tmp" // temporary files of store.WriteAtomic
)

var extKinds = map[string]Kind{
	".typ": KindTypst, ".md": KindMarkdown, ".markdown": KindMarkdown, ".txt": KindText,
	".csv": KindCSV, ".tsv": KindCSV, ".bib": KindBib, ".pdf": KindPDF,
	".png": KindImage, ".jpg": KindImage, ".jpeg": KindImage, ".gif": KindImage,
	".webp": KindImage, ".svg": KindImage, ".bmp": KindImage,
	".json": KindData, ".yaml": KindData, ".yml": KindData, ".toml": KindData,
	".xml": KindData, ".cbor": KindData,
}

// List returns the files of the project folder, not the folders: the main
// document, the user's own files and nothing of Inkwell's (.inkwell/, files
// and folders starting with a dot, temporary files). Symbolic links are
// followed only when they stay inside the project and lead to a file. In each
// folder the sub-folders come first, then the files, each in natural order
// ("ch2" before "ch10").
//
// The kind comes from the extension; a file without a known one is read
// briefly to tell text from the rest.
func (p *Project) List() ([]File, error) {
	var out []File
	if err := p.listDir(p.root, "", 0, &out); err != nil {
		return nil, fmt.Errorf("project: list: %w", err)
	}
	return out, nil
}

// listDir appends the files under dir, a root opened on the folder whose path
// from the project folder is prefix ("" for the project folder itself).
// Looking up each entry from its own folder keeps every path lookup to one
// component, which is what makes a 10000-file project list quickly.
func (p *Project) listDir(dir *os.Root, prefix string, depth int, out *[]File) error {
	if depth > maxListDepth {
		return nil
	}
	d, err := dir.Open(".")
	if err != nil {
		return err
	}
	entries, err := d.ReadDir(-1)
	if cerr := d.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int {
		switch {
		case naturalLess(a.Name(), b.Name()):
			return -1
		case naturalLess(b.Name(), a.Name()):
			return 1
		}
		return 0
	})
	var files []File
	for _, e := range entries {
		name := e.Name()
		if !listedName(name) {
			continue
		}
		rel := prefix + name
		if e.IsDir() {
			if err := p.listSub(dir, name, rel+"/", depth, out); err != nil {
				return err
			}
			continue
		}
		// Stat follows a link, but only inside the root: one that leaves it
		// (or dangles) fails and is left out, like a link to a folder.
		info, err := dir.Stat(name)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		files = append(files, File{Path: rel, Kind: p.kindOf(rel), Size: info.Size(), Main: rel == MainFile})
	}
	*out = append(*out, files...)
	return nil
}

// listedName reports whether [Project.List] shows an entry (file or folder)
// with this name: not Inkwell's, not hidden, not a temporary file.
func listedName(name string) bool {
	return !strings.HasPrefix(name, ".") && !strings.HasSuffix(name, tmpSuffix)
}

// listedPath reports whether [Project.List] can show the file rel (a
// [ValidatePath] path): every component is listed and the folder depth is
// within maxListDepth. The readers of the user's files accept no other path.
func listedPath(rel string) bool {
	depth := strings.Count(rel, "/")
	if depth > maxListDepth {
		return false
	}
	for part := range strings.SplitSeq(rel, "/") {
		if !listedName(part) {
			return false
		}
	}
	return true
}

// Version identifies the content of the listed file rel without reading it:
// its size and modification time. It changes when the file is edited outside
// Inkwell, and fails with [fs.ErrNotExist] once the file is deleted or
// renamed. The rules of the readers ([Project.ReadProjectText],
// [Project.ReadImage]) apply: a valid, listed, regular file.
func (p *Project) Version(rel string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ValidatePath(rel); err != nil {
		return "", err
	}
	if !listedPath(rel) {
		return "", fmt.Errorf("project: %s: %w", rel, fs.ErrNotExist)
	}
	info, err := p.lstatPath(rel)
	switch {
	case err != nil:
		return "", fmt.Errorf("project: %s: %w", rel, err)
	case !info.Mode().IsRegular():
		return "", fmt.Errorf("%w: %q", ErrNotRegular, rel)
	case hardLinks(info) > 1:
		return "", fmt.Errorf("%w: %q", ErrHardLink, rel)
	}
	return strconv.FormatInt(info.Size(), 10) + "-" + strconv.FormatInt(info.ModTime().UnixNano(), 10), nil
}

// listSub lists the folder name of dir.
func (p *Project) listSub(dir *os.Root, name, prefix string, depth int, out *[]File) error {
	sub, err := dir.OpenRoot(name)
	if err != nil {
		return err
	}
	err = p.listDir(sub, prefix, depth+1, out)
	return errors.Join(err, sub.Close())
}

// kindOf classifies rel by its extension, else by its content.
func (p *Project) kindOf(rel string) Kind {
	if k, ok := extKinds[strings.ToLower(path.Ext(rel))]; ok {
		return k
	}
	if p.looksLikeText(rel) {
		return KindText
	}
	return KindOther
}

// looksLikeText reads the start of rel and reports whether [Decode] accepts it.
func (p *Project) looksLikeText(rel string) bool {
	f, err := p.root.Open(rel)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }() // read-only handle
	head, err := io.ReadAll(io.LimitReader(f, sniffLen))
	if err != nil {
		return false
	}
	_, err = Decode(trimPartialRune(head))
	return err == nil
}

// trimPartialRune drops a UTF-8 sequence cut by the end of b, so a truncated
// read of a valid file is not mistaken for invalid UTF-8.
func trimPartialRune(b []byte) []byte {
	for i := 1; i <= min(utf8.UTFMax-1, len(b)); i++ {
		c := b[len(b)-i]
		if utf8.RuneStart(c) {
			if c >= 0x80 && !utf8.FullRune(b[len(b)-i:]) {
				return b[:len(b)-i]
			}
			break
		}
	}
	return b
}

// naturalLess orders names with numbers inside by value ("ch2" < "ch10") and
// ignores case, falling back to plain byte order so the order is total.
func naturalLess(a, b string) bool {
	for i, j := 0, 0; i < len(a) && j < len(b); {
		if isDigit(a[i]) && isDigit(b[j]) {
			ie, je := digitsEnd(a, i), digitsEnd(b, j)
			na, nb := strings.TrimLeft(a[i:ie], "0"), strings.TrimLeft(b[j:je], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if c := strings.Compare(na, nb); c != 0 {
				return c < 0
			}
			i, j = ie, je
			continue
		}
		if ca, cb := lower(a[i]), lower(b[j]); ca != cb {
			return ca < cb
		}
		i++
		j++
	}
	// One name is a prefix of the other, or they are equal up to case and
	// leading zeros: shorter first, then byte order.
	if c := cmp.Compare(len(a), len(b)); c != 0 {
		return c < 0
	}
	return a < b
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func lower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

func digitsEnd(s string, from int) int {
	for from < len(s) && isDigit(s[from]) {
		from++
	}
	return from
}

// ReadImage returns a bounded image projection. Access stays inside os.Root and
// excludes symlinks and private application files just like ReadText.
func (p *Project) ReadImage(rel string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ValidatePath(rel); err != nil {
		return nil, err
	}
	if extKinds[strings.ToLower(path.Ext(rel))] != KindImage {
		return nil, errors.New("project: not an image")
	}
	if !listedPath(rel) {
		return nil, fmt.Errorf("project: %s: %w", rel, fs.ErrNotExist)
	}
	if exists, err := p.regular(rel); err != nil {
		return nil, err
	} else if !exists {
		return nil, fs.ErrNotExist
	}
	return readLimited(p.root, rel, 64<<20)
}
