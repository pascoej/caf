package caf

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func TestBasicHelenKane(t *testing.T) {
	contents, err := ioutil.ReadFile("samples/helenkane.caf")
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) == 0 {
		t.Fatal("testing with empty file")
	}
	reader := bytes.NewReader(contents)
	f := &File{}
	if err := f.Decode(reader); err != nil {
		t.Fatal(err)
	}
	outputBuffer := &bytes.Buffer{}
	if err := f.Encode(outputBuffer); err != nil {
		t.Fatal(err)
	}
	if outputBuffer.Len() != len(contents) {
		t.Errorf("contents of input differ when decoding and reencoding, before: %d after: %d",
			len(contents),
			outputBuffer.Len())
	}
	output := outputBuffer.Bytes()
	for i := 0; i < len(contents); i++ {
		if output[i] != contents[i] {
			t.Errorf("contents of input differ when decoding and reencoding starting at offset %d", i)
			break
		}
	}
}