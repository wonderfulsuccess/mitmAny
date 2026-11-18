// Package i18n localizes application-owned native dialog strings.
// OS-generated authorization prompts continue to use the OS language.
package i18n

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
)

//go:embed messages.json
var data []byte
var messages map[string]string
var chinese atomic.Bool
var keys []string
var formatToken = regexp.MustCompile(`%[sdvw]`)

func init() {
	if err := json.Unmarshal(data, &messages); err != nil {
		panic(err)
	}
	for key := range messages {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
}
func SetLocale(locale string) error {
	switch locale {
	case "en":
		chinese.Store(false)
	case "zh-CN":
		chinese.Store(true)
	default:
		return fmt.Errorf("unsupported language: %s", locale)
	}
	return nil
}
func Text(source string) string {
	if !chinese.Load() {
		if en, ok := messages[source]; ok {
			return en
		}
	}
	return source
}
func Diagnostic(message string) string { return diagnostic(message, 0) }
func diagnostic(message string, depth int) string {
	if depth > 8 {
		return message
	}
	for _, source := range keys {
		target := Text(source)
		if source == message {
			return target
		}
		if strings.HasSuffix(source, ": ") && strings.HasPrefix(message, source) {
			return target + diagnostic(strings.TrimPrefix(message, source), depth+1)
		}
		if !formatToken.MatchString(source) {
			continue
		}
		parts := formatToken.Split(source, -1)
		for i := range parts {
			parts[i] = regexp.QuoteMeta(parts[i])
		}
		match := regexp.MustCompile("(?s)^" + strings.Join(parts, "(.*?)") + "$").FindStringSubmatch(message)
		if match != nil {
			i := 0
			return formatToken.ReplaceAllStringFunc(target, func(string) string { i++; return diagnostic(match[i], depth+1) })
		}
	}
	if strings.Contains(message, "\n") {
		parts := strings.Split(message, "\n")
		for i := range parts {
			parts[i] = diagnostic(parts[i], depth+1)
		}
		return strings.Join(parts, "\n")
	}
	return message
}
