package typst

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// maxAttachmentBytes bounds what is inflated for one attachment, so that a
// PDF cannot turn a few kilobytes into gigabytes. A variable for the tests.
var maxAttachmentBytes = 256 << 20

const (
	maxPDFObjects = 200_000
	maxNesting    = 32
)

// Attachment is a file embedded in a PDF (Typst's pdf.attach).
type Attachment struct {
	// Name is the file name given to pdf.attach.
	Name string
	// Description is the optional description.
	Description string
	// MIMEType is the optional media type, e.g. "text/csv".
	MIMEType string
	// Data is the content of the file, inflated.
	Data []byte
}

var (
	objectRE = regexp.MustCompile(`(?:^|[\r\n\t ])(\d{1,9}) (\d{1,5}) obj\b`)
	refRE    = regexp.MustCompile(`^(\d{1,9})\s+\d{1,5}\s+R$`)
	lengthRE = regexp.MustCompile(`^(\d{1,15})$`)
	// "12 0 R" at the start of a value, followed by a delimiter.
	refPrefixRE = regexp.MustCompile(`^(\d{1,9}\s+\d{1,5}\s+R)(?:[\s/<>\[\]()%]|$)`)
)

// pdfObject is one indirect object: its dictionary and, if any, its stream.
type pdfObject struct {
	dict      map[string]string // top-level keys (without "/") to raw values
	stream    []byte
	hasStream bool
}

// Attachments lists the files embedded in a PDF, in file order. It reads what
// Typst writes (plain objects, no object streams) and refuses anything it
// cannot read with certainty, rather than guessing. Streams are inflated up to
// a bound. It is how Inkwell checks that pdf.attach reached the PDF.
func Attachments(pdf []byte) ([]Attachment, error) {
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		return nil, errors.New("typst: not a PDF")
	}
	objs, order, err := readObjects(pdf)
	if err != nil {
		return nil, err
	}
	var out []Attachment
	for _, num := range order {
		spec := objs[num]
		if spec.dict["Type"] != "/Filespec" {
			continue
		}
		a, err := attachmentOf(spec, objs)
		if err != nil {
			return nil, fmt.Errorf("typst: attachment in object %d: %w", num, err)
		}
		out = append(out, a)
	}
	return out, nil
}

func attachmentOf(spec pdfObject, objs map[int]pdfObject) (Attachment, error) {
	var a Attachment
	a.Name = decodeString(spec.dict["UF"])
	if a.Name == "" {
		a.Name = decodeString(spec.dict["F"])
	}
	a.Description = decodeString(spec.dict["Desc"])

	ef, _, err := parseDict([]byte(spec.dict["EF"]), 0)
	if err != nil || ef == nil {
		return a, errors.New("no embedded file stream")
	}
	ref := ef["F"]
	if ref == "" {
		ref = ef["UF"]
	}
	m := refRE.FindStringSubmatch(ref)
	if m == nil {
		return a, errors.New("embedded file is not a reference")
	}
	num, _ := strconv.Atoi(m[1])
	file, ok := objs[num]
	if !ok || !file.hasStream {
		return a, fmt.Errorf("embedded file object %d has no stream", num)
	}
	a.MIMEType = decodeName(file.dict["Subtype"])
	a.Data, err = decodeStream(file)
	return a, err
}

// readObjects finds every indirect object. Stream bodies are skipped by their
// declared length, so data inside a stream is never taken for an object.
func readObjects(pdf []byte) (map[int]pdfObject, []int, error) {
	objs := map[int]pdfObject{}
	var order []int
	for pos := 0; pos < len(pdf); {
		loc := objectRE.FindSubmatchIndex(pdf[pos:])
		if loc == nil {
			break
		}
		num, _ := strconv.Atoi(string(pdf[pos+loc[2] : pos+loc[3]]))
		obj, end, err := readObject(pdf, pos+loc[1])
		if err != nil {
			return nil, nil, fmt.Errorf("typst: PDF object %d: %w", num, err)
		}
		if len(order) >= maxPDFObjects {
			return nil, nil, errors.New("typst: PDF has too many objects")
		}
		if _, dup := objs[num]; !dup {
			order = append(order, num)
		}
		objs[num] = obj
		pos = end
	}
	return objs, order, nil
}

// readObject parses the object whose body starts at start and returns it with
// the offset just after it.
func readObject(pdf []byte, start int) (pdfObject, int, error) {
	i := skipSpace(pdf, start)
	if !bytes.HasPrefix(pdf[i:], []byte("<<")) {
		end := bytes.Index(pdf[i:], []byte("endobj"))
		if end < 0 {
			return pdfObject{}, 0, errors.New("unterminated")
		}
		return pdfObject{}, i + end, nil
	}
	dict, dictEnd, err := parseDict(pdf, i)
	if err != nil {
		return pdfObject{}, 0, err
	}
	obj := pdfObject{dict: dict}
	j := skipSpace(pdf, dictEnd)
	if !bytes.HasPrefix(pdf[j:], []byte("stream")) {
		return obj, dictEnd, nil
	}
	j += len("stream")
	switch {
	case bytes.HasPrefix(pdf[j:], []byte("\r\n")):
		j += 2
	case j < len(pdf) && (pdf[j] == '\n' || pdf[j] == '\r'):
		j++
	}
	var n int
	if m := lengthRE.FindStringSubmatch(dict["Length"]); m != nil {
		n, err = strconv.Atoi(m[1])
		if err != nil || n > len(pdf)-j {
			return pdfObject{}, 0, errors.New("stream length overruns the file")
		}
	} else { // indirect length: look for the end of the stream
		k := bytes.Index(pdf[j:], []byte("endstream"))
		if k < 0 {
			return pdfObject{}, 0, errors.New("unterminated stream")
		}
		n = len(bytes.TrimRight(pdf[j:j+k], "\r\n"))
	}
	end := skipSpace(pdf, j+n)
	if !bytes.HasPrefix(pdf[end:], []byte("endstream")) {
		return pdfObject{}, 0, errors.New("stream does not end where its length says")
	}
	obj.stream, obj.hasStream = pdf[j:j+n], true
	return obj, end + len("endstream"), nil
}

// decodeStream applies the stream's filters; only FlateDecode is understood.
func decodeStream(o pdfObject) ([]byte, error) {
	filter := o.dict["Filter"]
	switch {
	case filter == "":
		if len(o.stream) > maxAttachmentBytes {
			return nil, errors.New("attachment is larger than the bound")
		}
		return bytes.Clone(o.stream), nil
	case filter == "/FlateDecode" || bytes.Equal(bytes.Join(bytes.Fields([]byte(filter)), nil), []byte("[/FlateDecode]")):
		zr, err := zlib.NewReader(bytes.NewReader(o.stream))
		if err != nil {
			return nil, fmt.Errorf("inflate: %w", err)
		}
		defer func() { _ = zr.Close() }()
		data, err := io.ReadAll(io.LimitReader(zr, int64(maxAttachmentBytes)+1))
		if err != nil {
			return nil, fmt.Errorf("inflate: %w", err)
		}
		if len(data) > maxAttachmentBytes {
			return nil, errors.New("attachment inflates past the bound")
		}
		return data, nil
	}
	return nil, fmt.Errorf("unsupported stream filter %s", filter)
}

func skipSpace(b []byte, i int) int {
	for i < len(b) && isSpace(b[i]) {
		i++
	}
	return i
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\n' || c == '\r' || c == '\t' || c == '\f' || c == 0
}

func isDelimiter(c byte) bool {
	return isSpace(c) || bytes.IndexByte([]byte("()<>[]{}/%"), c) >= 0
}

// parseDict reads the dictionary at b[i:] and returns its top-level entries,
// the key without its slash and the raw text of the value, plus the offset
// after it. depth bounds the nesting.
func parseDict(b []byte, i int) (map[string]string, int, error) {
	if len(b) == 0 {
		return nil, 0, nil
	}
	return parseDictDepth(b, i, 0)
}

func parseDictDepth(b []byte, i, depth int) (map[string]string, int, error) {
	if depth > maxNesting {
		return nil, 0, errors.New("dictionary nested too deeply")
	}
	if !bytes.HasPrefix(b[i:], []byte("<<")) {
		return nil, 0, errors.New("not a dictionary")
	}
	dict := map[string]string{}
	p := i + 2
	for {
		p = skipSpace(b, p)
		switch {
		case p >= len(b):
			return nil, 0, errors.New("unterminated dictionary")
		case bytes.HasPrefix(b[p:], []byte(">>")):
			return dict, p + 2, nil
		case b[p] != '/':
			return nil, 0, errors.New("dictionary key is not a name")
		}
		keyEnd := p + 1
		for keyEnd < len(b) && !isDelimiter(b[keyEnd]) {
			keyEnd++
		}
		key := string(b[p+1 : keyEnd])
		vStart := skipSpace(b, keyEnd)
		vEnd, err := skipValue(b, vStart, depth+1)
		if err != nil {
			return nil, 0, err
		}
		if _, dup := dict[key]; !dup {
			dict[key] = string(bytes.TrimSpace(b[vStart:vEnd]))
		}
		p = vEnd
	}
}

// skipValue returns the offset after the value starting at b[p].
func skipValue(b []byte, p, depth int) (int, error) {
	if p >= len(b) {
		return 0, errors.New("missing value")
	}
	switch c := b[p]; {
	case c == '<' && bytes.HasPrefix(b[p:], []byte("<<")):
		_, end, err := parseDictDepth(b, p, depth)
		return end, err
	case c == '<':
		k := bytes.IndexByte(b[p:], '>')
		if k < 0 {
			return 0, errors.New("unterminated hex string")
		}
		return p + k + 1, nil
	case c == '(':
		return skipLiteral(b, p)
	case c == '[':
		if depth > maxNesting {
			return 0, errors.New("array nested too deeply")
		}
		q := p + 1
		for {
			q = skipSpace(b, q)
			if q >= len(b) {
				return 0, errors.New("unterminated array")
			}
			if b[q] == ']' {
				return q + 1, nil
			}
			var err error
			if q, err = skipValue(b, q, depth+1); err != nil {
				return 0, err
			}
		}
	case c == '/':
		q := p + 1
		for q < len(b) && !isDelimiter(b[q]) {
			q++
		}
		return q, nil
	}
	// A number, a reference "12 0 R", a keyword.
	q := p
	for q < len(b) && !isDelimiter(b[q]) {
		q++
	}
	if q == p {
		return 0, fmt.Errorf("unexpected %q", b[p])
	}
	if loc := refPrefixRE.FindSubmatchIndex(b[p:min(p+48, len(b))]); loc != nil {
		return p + loc[3], nil
	}
	return q, nil
}

// skipLiteral returns the offset after the literal string starting at b[p] == '('.
func skipLiteral(b []byte, p int) (int, error) {
	depth := 0
	for i := p; i < len(b); i++ {
		switch b[i] {
		case '\\':
			i++
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i + 1, nil
			}
		}
	}
	return 0, errors.New("unterminated string")
}

// decodeName turns a PDF name such as "/text#2Fcsv" into "text/csv".
func decodeName(raw string) string {
	if len(raw) == 0 || raw[0] != '/' {
		return ""
	}
	var sb []byte
	for i := 1; i < len(raw); i++ {
		if raw[i] == '#' && i+2 < len(raw) {
			if v, err := strconv.ParseUint(raw[i+1:i+3], 16, 8); err == nil {
				sb = append(sb, byte(v))
				i += 2
				continue
			}
		}
		sb = append(sb, raw[i])
	}
	return string(sb)
}

// decodeString decodes a PDF literal or hexadecimal text string. A leading
// FE FF byte order mark means UTF-16BE; otherwise the bytes are UTF-8 when
// valid, and Latin-1 when not.
func decodeString(raw string) string {
	var data []byte
	switch {
	case len(raw) >= 2 && raw[0] == '(' && raw[len(raw)-1] == ')':
		data = unescapeLiteral(raw[1 : len(raw)-1])
	case len(raw) >= 2 && raw[0] == '<' && raw[len(raw)-1] == '>':
		data = unhex(raw[1 : len(raw)-1])
	default:
		return ""
	}
	if len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF {
		units := make([]uint16, 0, len(data)/2)
		for i := 2; i+1 < len(data); i += 2 {
			units = append(units, uint16(data[i])<<8|uint16(data[i+1]))
		}
		return string(utf16.Decode(units))
	}
	if utf8.Valid(data) {
		return string(data)
	}
	runes := make([]rune, len(data))
	for i, c := range data {
		runes[i] = rune(c)
	}
	return string(runes)
}

func unescapeLiteral(s string) []byte {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			out = append(out, c)
			continue
		}
		i++
		switch e := s[i]; e {
		case 'n':
			out = append(out, '\n')
		case 'r':
			out = append(out, '\r')
		case 't':
			out = append(out, '\t')
		case 'b':
			out = append(out, '\b')
		case 'f':
			out = append(out, '\f')
		case '\n':
			// line continuation
		case '\r':
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
		case '0', '1', '2', '3', '4', '5', '6', '7':
			v, n := int(e-'0'), 1
			for n < 3 && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '7' {
				i++
				v = v*8 + int(s[i]-'0')
				n++
			}
			out = append(out, byte(v&0xff)) // PDF: high-order overflow is ignored
		default:
			out = append(out, e)
		}
	}
	return out
}

func unhex(s string) []byte {
	digits := make([]byte, 0, len(s))
	for i := range len(s) {
		if !isSpace(s[i]) {
			digits = append(digits, s[i])
		}
	}
	if len(digits)%2 == 1 {
		digits = append(digits, '0')
	}
	out := make([]byte, 0, len(digits)/2)
	for i := 0; i+1 < len(digits); i += 2 {
		v, err := strconv.ParseUint(string(digits[i:i+2]), 16, 8)
		if err != nil {
			return out
		}
		out = append(out, byte(v))
	}
	return out
}
