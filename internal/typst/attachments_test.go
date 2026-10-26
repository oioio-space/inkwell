package typst

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/google/go-cmp/cmp"
)

// pdf assembles a minimal file: header, the objects numbered from 1, trailer.
func pdf(objects ...string) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n%\x80\x80\x80\x80\n")
	for i, o := range objects {
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	b.WriteString("trailer\n<</Size 99>>\n%%EOF\n")
	return b.Bytes()
}

func stream(dict, data string) string {
	return fmt.Sprintf("<<%s/Length %d>>\nstream\n%s\nendstream", dict, len(data), data)
}

func deflate(t testing.TB, data []byte) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func filespec(name string, ref int, extra string) string {
	return fmt.Sprintf("<</Type/Filespec/F(%s)/UF(%s)/EF<</F %d 0 R/UF %d 0 R>>%s>>", name, name, ref, ref, extra)
}

func TestAttachments(t *testing.T) {
	csv := "a,b\n1,2\n"
	t.Run("a plain stream with its name, description and type", func(t *testing.T) {
		file := pdf(
			filespec("data.csv", 2, "/Desc(the data)"),
			stream("/Type/EmbeddedFile/Subtype/text#2Fcsv/Params<</Size 8>>", csv),
		)
		got, err := Attachments(file)
		if err != nil {
			t.Fatalf("Attachments = %v", err)
		}
		want := []Attachment{{Name: "data.csv", Description: "the data", MIMEType: "text/csv", Data: []byte(csv)}}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Attachments mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("a deflated stream", func(t *testing.T) {
		payload := bytes.Repeat([]byte("compressible "), 500)
		file := pdf(
			filespec("big.txt", 2, ""),
			stream("/Type/EmbeddedFile/Filter/FlateDecode", deflate(t, payload)),
		)
		got, err := Attachments(file)
		if err != nil || len(got) != 1 || !bytes.Equal(got[0].Data, payload) {
			t.Fatalf("Attachments = %v, %v; want the inflated payload", got, err)
		}
	})

	t.Run("a filter array and an indirect length", func(t *testing.T) {
		payload := []byte("indirect")
		z := deflate(t, payload)
		file := pdf(
			filespec("n.txt", 2, ""),
			fmt.Sprintf("<</Type/EmbeddedFile/Filter[/FlateDecode]/Length 3 0 R>>\nstream\n%s\nendstream", z),
			fmt.Sprintf("%d", len(z)),
		)
		got, err := Attachments(file)
		if err != nil || len(got) != 1 || !bytes.Equal(got[0].Data, payload) {
			t.Fatalf("Attachments = %v, %v", got, err)
		}
	})

	t.Run("names with escapes, hex and UTF-16", func(t *testing.T) {
		utf16Hex := func(s string) string {
			var sb strings.Builder
			sb.WriteString("FEFF")
			for _, u := range utf16.Encode([]rune(s)) {
				fmt.Fprintf(&sb, "%04X", u)
			}
			return sb.String()
		}
		file := pdf(
			`<</Type/Filespec/F(a\(1\)\\b\101.txt)/EF<</F 3 0 R>>>>`,
			`<</Type/Filespec/UF<`+utf16Hex("é日本😀.csv")+`>/F(ignored.csv)/EF<</F 3 0 R>>>>`,
			stream("/Type/EmbeddedFile", "x"),
		)
		got, err := Attachments(file)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, a := range got {
			names = append(names, a.Name)
		}
		if want := []string{`a(1)\bA.txt`, "é日本😀.csv"}; !cmp.Equal(names, want) {
			t.Errorf("names = %q, want %q", names, want)
		}
	})

	t.Run("a PDF without attachment", func(t *testing.T) {
		got, err := Attachments(pdf("<</Type/Catalog>>"))
		if err != nil || len(got) != 0 {
			t.Errorf("Attachments = %v, %v; want none", got, err)
		}
	})

	t.Run("binary data that looks like a PDF object does not fool the scan", func(t *testing.T) {
		// An attached file that contains "9 0 obj" must not become an object.
		payload := "line\n9 0 obj\n<</Type/Filespec/F(fake.txt)/EF<</F 2 0 R>>>>\nendobj\n"
		file := pdf(filespec("real.txt", 2, ""), stream("/Type/EmbeddedFile", payload))
		got, err := Attachments(file)
		if err != nil || len(got) != 1 || got[0].Name != "real.txt" || string(got[0].Data) != payload {
			t.Fatalf("Attachments = %+v, %v; want only real.txt with the payload intact", got, err)
		}
	})

	t.Run("refused inputs", func(t *testing.T) {
		old := maxAttachmentBytes
		maxAttachmentBytes = 1 << 20
		t.Cleanup(func() { maxAttachmentBytes = old })

		tests := map[string][]byte{
			"empty":           nil,
			"not a PDF":       []byte("hello"),
			"missing stream":  pdf(filespec("a.txt", 2, "")),
			"unknown filter":  pdf(filespec("a.txt", 2, ""), stream("/Type/EmbeddedFile/Filter/LZWDecode", "x")),
			"corrupt deflate": pdf(filespec("a.txt", 2, ""), stream("/Type/EmbeddedFile/Filter/FlateDecode", "not zlib")),
			"inflation bomb":  pdf(filespec("a.txt", 2, ""), stream("/Type/EmbeddedFile/Filter/FlateDecode", deflate(t, make([]byte, 8<<20)))),
			"length overruns": []byte("%PDF-1.7\n1 0 obj\n<</Type/EmbeddedFile/Length 99999>>\nstream\nabc\nendstream\nendobj\n"),
		}
		for name, in := range tests {
			t.Run(name, func(t *testing.T) {
				if got, err := Attachments(in); err == nil {
					t.Errorf("Attachments = %+v, nil; want an error", got)
				}
			})
		}
	})
}

func FuzzAttachments(f *testing.F) {
	f.Add(pdf(filespec("a.txt", 2, "/Desc(x)"), stream("/Type/EmbeddedFile", "data")))
	f.Add(pdf(`<</Type/Filespec/F<FEFF0041>/EF<</F 1 0 R>>>>`))
	f.Add([]byte("%PDF-1.7\n1 0 obj\n<<<<<<<<(((((\nstream\n"))
	f.Add([]byte("%PDF-1.7\n1 0 obj\n<</Length 99999999999999999999>>\nstream\n"))
	f.Fuzz(func(t *testing.T, in []byte) {
		got, err := Attachments(in)
		if err != nil {
			return
		}
		total := 0
		for _, a := range got {
			total += len(a.Data)
		}
		if total > maxAttachmentBytes {
			t.Fatalf("returned %d bytes, over the bound", total)
		}
	})
}

func TestAttachmentsTotalBound(t *testing.T) {
	oldOne, oldAll := maxAttachmentBytes, maxTotalAttachmentBytes
	maxAttachmentBytes, maxTotalAttachmentBytes = 1<<20, 1500<<10
	t.Cleanup(func() { maxAttachmentBytes, maxTotalAttachmentBytes = oldOne, oldAll })

	one := bytes.Repeat([]byte("a"), 1000<<10) // each under the per-file bound
	file := pdf(
		filespec("a.bin", 3, ""), filespec("b.bin", 4, ""),
		stream("/Type/EmbeddedFile", string(one)), stream("/Type/EmbeddedFile", string(one)),
	)
	if got, err := Attachments(file); err == nil {
		t.Errorf("Attachments = %d files, nil; want an error past the total bound", len(got))
	}
	if got, err := Attachments(pdf(filespec("a.bin", 2, ""), stream("/Type/EmbeddedFile", string(one)))); err != nil || len(got) != 1 {
		t.Errorf("a single attachment under both bounds: %v, %v", got, err)
	}
}
