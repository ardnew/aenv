package log

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestAppendJSONString_MatchesEncodingJSON locks the direct JSON string encoder
// to byte-identical output with encoding/json's default (HTML-escaping) encoder.
func TestAppendJSONString_MatchesEncodingJSON(t *testing.T) {
	cases := []string{
		"",
		"plain",
		"with space",
		"quote\"inside",
		"back\\slash",
		"tab\tnewline\nreturn\r",
		"control\x00\x01\x1f\x7f",
		"html<>&chars",
		"unicode: \u00e9\u2603",
		"line sep\u2028 para\u2029",
		"invalid\xffutf8",
		"mixed <a href=\"x\">&\u2028\t",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			want, err := json.Marshal(in)
			if err != nil {
				t.Fatalf("json.Marshal(%q) error = %v", in, err)
			}
			var buf bytes.Buffer
			appendJSONString(&buf, in)
			if got := buf.Bytes(); !bytes.Equal(got, want) {
				t.Fatalf("appendJSONString(%q)\nwant: %s\ngot:  %s", in, want, got)
			}
		})
	}
}
