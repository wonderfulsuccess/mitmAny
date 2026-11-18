package i18n

import "testing"

func TestLanguage(t *testing.T) {
	defer SetLocale("en")
	if got := Text("所有文件"); got != "All files" {
		t.Fatalf("default: %q", got)
	}
	if err := SetLocale("zh-CN"); err != nil {
		t.Fatal(err)
	}
	if got := Text("所有文件"); got != "所有文件" {
		t.Fatalf("Chinese: %q", got)
	}
	if err := SetLocale("en"); err != nil {
		t.Fatal(err)
	}
	if got := Diagnostic("代理端口 9080 不可用: address already in use"); got != "Proxy port 9080 is unavailable: address already in use" {
		t.Fatalf("diagnostic: %q", got)
	}
	if err := SetLocale("unknown"); err == nil {
		t.Fatal("invalid locale accepted")
	}
}
