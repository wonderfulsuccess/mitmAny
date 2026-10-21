package capture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDomainNames(t *testing.T) {
	for _, tc := range []struct{ host, domain, root string }{
		{"wx.qlogo.cn:443", "wx.qlogo.cn", "qlogo.cn"},
		{"MP.WeiXin.QQ.COM.", "mp.weixin.qq.com", "qq.com"},
		{"api.example.co.uk:8443", "api.example.co.uk", "example.co.uk"},
		{"api.example.com.cn", "api.example.com.cn", "example.com.cn"},
		{"foo.github.io", "foo.github.io", "foo.github.io"},
		{"127.0.0.1:8080", "127.0.0.1", "127.0.0.1"},
		{"[2001:db8::1]:443", "2001:db8::1", "2001:db8::1"},
		{"2001:db8::1", "2001:db8::1", "2001:db8::1"},
		{"localhost:9000", "localhost", "localhost"},
	} {
		t.Run(tc.host, func(t *testing.T) {
			domain, root := DomainNames(tc.host)
			if domain != tc.domain || root != tc.root {
				t.Fatalf("got %q/%q, want %q/%q", domain, root, tc.domain, tc.root)
			}
		})
	}
}
func TestSavedSessionsGainDomainGroups(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "session.json"), []byte(`[{"id":"saved","host":"api.example.co.uk:443","url":"https://api.example.co.uk/","complete":true}]`), 0600)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	batch := store.Batch(0, 0)
	if len(batch.Flows) != 1 || batch.Flows[0].RootDomain != "example.co.uk" || batch.Flows[0].Domain != "api.example.co.uk" {
		t.Fatalf("old session did not gain domain groups: %+v", batch)
	}
}
