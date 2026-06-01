package llm

import "testing"

func TestParseModelJSON(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantOK  bool
		wantSub string
	}{
		{
			name:    "raw object",
			in:      `{"type":"todo","confidence":0.9}`,
			wantOK:  true,
			wantSub: `"type":"todo"`,
		},
		{
			name:    "fenced",
			in:      "```json\n{\"items\":[]}\n```",
			wantOK:  true,
			wantSub: `"items"`,
		},
		{
			name:    "prose then object",
			in:      "Let me extract the todo list:\n\n{\"items\":[{\"task\":\"Buy milk\"}]}",
			wantOK:  true,
			wantSub: `"Buy milk"`,
		},
		{
			name:   "plain prose",
			in:     "Here is your list of tasks...",
			wantOK: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseModelJSON(tc.in)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v want %v", ok, tc.wantOK)
			}
			if tc.wantOK && !contains(string(got), tc.wantSub) {
				t.Fatalf("got %q want substring %q", got, tc.wantSub)
			}
		})
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && searchSub(s, sub))
}

func searchSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
