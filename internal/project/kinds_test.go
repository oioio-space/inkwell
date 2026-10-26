package project

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		want Kind
		ok   bool
	}{
		{"main.typ", KindTypst, true},
		{"Notes.MD", KindMarkdown, true},
		{"data.tsv", KindCSV, true},
		{"app.log", KindText, true},
		{"server.conf", KindText, true},
		{"x/y/script.sh", KindText, true},
		{"config.yaml", KindData, true},
		{"README", KindText, true},
		{"sub/Makefile", KindText, true},
		{"logo.PNG", KindImage, true},
		{"font.ttf", KindOther, false}, // not in the table: sniffed, then binary
		{"archive.zip", KindOther, false},
		{"noextension", KindOther, false},
	}
	for _, tt := range tests {
		got, ok := classify(tt.name)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("classify(%q) = %v, %v; want %v, %v", tt.name, got, ok, tt.want, tt.ok)
		}
	}
}

// The numeric values are the wire format of the store stream: the table maps onto them.
func TestKindValuesAreTheWireFormat(t *testing.T) {
	want := map[Kind]uint8{
		KindOther: 0, KindTypst: 1, KindMarkdown: 2, KindText: 3, KindCSV: 4,
		KindBib: 5, KindImage: 6, KindPDF: 7, KindData: 8,
	}
	for k, v := range want {
		if uint8(k) != v {
			t.Errorf("%v = %d, want %d", k, k, v)
		}
	}
}
