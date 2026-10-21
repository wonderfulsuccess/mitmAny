//go:build darwin && !cgo

package certificates

import "errors"

func nativeCertificate([]byte, bool) error {
	return errors.New("macOS 证书安装需要启用 CGO 的桌面构建；可使用钥匙串完成安装")
}
