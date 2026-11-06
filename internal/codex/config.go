package codex

import (
	"bytes"
	"errors"
	"reflect"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
)

func parse(data []byte) (map[string]any, error) {
	m := map[string]any{}
	err := toml.Unmarshal(data, &m)
	return m, err
}
func table(m map[string]any, k string) map[string]any { v, _ := m[k].(map[string]any); return v }
func isTarget(path []string) bool {
	return len(path) >= 2 && path[0] == "mcp_servers" && path[1] == "mitmany"
}
func keys(n *unstable.Node) []string {
	out := []string{}
	it := n.Key()
	for it.Next() {
		out = append(out, string(it.Node().Data))
	}
	return out
}

// replaceServer removes only expressions belonging to this MCP entry, retaining
// all unrelated bytes (including comments and multiline string contents).
func replaceServer(data []byte, entry map[string]any) ([]byte, error) {
	before, err := parse(data)
	if err != nil {
		return nil, errors.New("Codex config.toml 格式无效，请先修复配置文件")
	}
	var p unstable.Parser
	p.Reset(data)
	prefix := []string{}
	type span struct{ start, end int }
	spans := []span{}
	for p.NextExpression() {
		n := p.Expression()
		remove := false
		switch n.Kind {
		case unstable.Table, unstable.ArrayTable:
			prefix = keys(n)
			remove = isTarget(prefix)
		case unstable.KeyValue:
			path := append(append([]string{}, prefix...), keys(n)...)
			if len(path) == 1 && path[0] == "mcp_servers" {
				return nil, errors.New("请先将 config.toml 中内联的 mcp_servers 改为 [mcp_servers.名称] 表格式，再连接")
			}
			remove = isTarget(path)
		}
		if remove {
			start, end := int(n.Raw.Offset), int(n.Raw.Offset+n.Raw.Length)
			if n.Kind == unstable.Table || n.Kind == unstable.ArrayTable {
				it := n.Key()
				if !it.Next() {
					return nil, errors.New("配置表缺少名称")
				}
				keyOffset := int(it.Node().Raw.Offset)
				start = bytes.LastIndexByte(data[:keyOffset], '\n') + 1
				end = len(data)
				if next := bytes.IndexByte(data[keyOffset:], '\n'); next >= 0 {
					end = keyOffset + next
				}
			}
			if start < 0 || end <= start || end > len(data) || (len(spans) > 0 && start < spans[len(spans)-1].end) {
				return nil, errors.New("无法确定配置修改范围；原文件未修改")
			}
			spans = append(spans, span{start, end})
		}
	}
	if p.Error() != nil {
		return nil, p.Error()
	}
	var out bytes.Buffer
	pos := 0
	for _, r := range spans {
		out.Write(data[pos:r.start])
		pos = r.end
	}
	out.Write(data[pos:])
	out.WriteString("\n")
	added, err := toml.Marshal(entry)
	if err != nil {
		return nil, err
	}
	out.WriteString("[mcp_servers.mitmany]\n")
	out.Write(added)
	after, err := parse(out.Bytes())
	if err != nil {
		return nil, errors.New("无法安全更新 Codex MCP 配置；原文件未修改")
	}
	// A final semantic check guards against parser range changes and unusual TOML.
	delete(table(before, "mcp_servers"), "mitmany")
	delete(table(after, "mcp_servers"), "mitmany")
	if len(table(before, "mcp_servers")) == 0 {
		delete(before, "mcp_servers")
	}
	if len(table(after, "mcp_servers")) == 0 {
		delete(after, "mcp_servers")
	}
	if !reflect.DeepEqual(before, after) {
		return nil, errors.New("配置更新会影响其他设置，已取消；原文件未修改")
	}
	return out.Bytes(), nil
}
