package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadDecodeStrict(t *testing.T) {
	for _, tc := range []struct {
		body   string
		status int
		seq    int64
	}{
		{`{"through_seq":"0"}`, 200, 0}, {`{"through_seq":"9223372036854775807"}`, 200, 9223372036854775807},
		{`{}`, 422, 0}, {`{"through_seq":null}`, 422, 0}, {`{"through_seq":1}`, 400, 0}, {`{"through_seq":true}`, 400, 0},
		{`{"through_seq":[]}`, 400, 0}, {`{"through_seq":{}}`, 400, 0}, {`{"through_seq":"-1"}`, 422, 0},
		{`{"through_seq":"01"}`, 422, 0}, {`{"through_seq":"+1"}`, 422, 0}, {`{"through_seq":"1e0"}`, 422, 0},
		{`{"through_seq":"9223372036854775808"}`, 422, 0}, {`{"through_seq":""}`, 422, 0},
		{`{"through_seq":" 1"}`, 422, 0}, {`{"through_seq":"-0"}`, 422, 0},
		{`{"through_seq":"1","through_seq":"1"}`, 400, 0}, {`{"through_seq":"1","\u0074hrough_seq":"2"}`, 400, 0},
		{`{"Through_seq":"1"}`, 400, 0}, {`{"through_seq":"1","extra":0}`, 400, 0}, {`{"through_seq":"1"} {}`, 400, 0},
		{`null`, 400, 0}, {`[]`, 400, 0}, {`"x"`, 400, 0}, {`{"through_seq":`, 400, 0},
		{strings.Repeat(" ", 262144) + `{"through_seq":"0"}`, 413, 0},
		{`{"through_seq":"0"}` + strings.Repeat(" ", 262144-len(`{"through_seq":"0"}`)), 200, 0},
	} {
		name := tc.body
		if len(name) > 100 {
			name = name[:100]
		}
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest("PUT", "/", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			seq, ok := decodeRead(w, r)
			if w.Code != tc.status || ok != (tc.status == 200) || ok && seq != tc.seq {
				t.Fatalf("status=%d ok=%v seq=%d", w.Code, ok, seq)
			}
		})
	}
}
