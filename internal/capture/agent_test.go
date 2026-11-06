package capture

import (
	"os"
	"strings"
	"testing"
)

func TestAgentBodyBoundsHTMLAndMissingFiles(t *testing.T) {
	s, e := NewStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	d := &Detail{Summary: Summary{ID: "test", ContentType: "text/html", Complete: true}, ResponseCaptured: PreviewLimit + 100}
	s.entries[d.ID] = d
	if e = os.WriteFile(s.bodyPath(d.ID, false), []byte(strings.Repeat("x", PreviewLimit+100)), 0600); e != nil {
		t.Fatal(e)
	}
	data, encoding, limited, e := s.AgentBody(d.ID, false)
	if e != nil || !limited || len(data) != PreviewLimit || encoding != "base64" {
		t.Fatal(len(data), encoding, limited, e)
	}
	os.Remove(s.bodyPath(d.ID, false))
	if _, _, _, e = s.AgentBody(d.ID, false); e == nil {
		t.Fatal("missing captured body treated as empty")
	}
	if _, e = s.Metadata("../../outside"); e == nil {
		t.Fatal("unindexed ID accepted")
	}
}
