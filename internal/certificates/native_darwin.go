//go:build darwin && cgo

package certificates

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <Security/Security.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>

static OSStatus mitmany_certificate(const void *bytes, long length, int remove) {
    if (length <= 0) return errSecDecode;
    CFDataRef data = CFDataCreate(NULL, bytes, length);
    if (data == NULL) return errSecAllocate;
    SecCertificateRef certificate = SecCertificateCreateWithData(NULL, data);
    CFRelease(data);
    if (certificate == NULL) return errSecDecode;

    SecKeychainRef keychain = NULL;
    OSStatus status = SecKeychainCopyDefault(&keychain);
    if (status != errSecSuccess) { CFRelease(certificate); return status; }
    if (remove) {
        status = SecTrustSettingsRemoveTrustSettings(certificate, kSecTrustSettingsDomainUser);
        if (status == errSecItemNotFound || status == errSecNoTrustSettings) status = errSecSuccess;
        if (status == errSecSuccess) {
            // Remove any trust left by older system-keychain installations too.
            CFArrayRef adminSettings = NULL;
            OSStatus adminStatus = SecTrustSettingsCopyTrustSettings(certificate, kSecTrustSettingsDomainAdmin, &adminSettings);
            if (adminStatus == errSecSuccess) {
                if (adminSettings != NULL) CFRelease(adminSettings);
                status = SecTrustSettingsRemoveTrustSettings(certificate, kSecTrustSettingsDomainAdmin);
            } else if (adminStatus != errSecItemNotFound && adminStatus != errSecNoTrustSettings) {
                status = adminStatus;
            }
        }
        if (status == errSecSuccess) {
            const void *items[] = {certificate};
            CFArrayRef list = CFArrayCreate(NULL, items, 1, &kCFTypeArrayCallBacks);
            const void *keychains[] = {keychain};
            CFArrayRef search = CFArrayCreate(NULL, keychains, 1, &kCFTypeArrayCallBacks);
            const void *keys[] = {kSecClass, kSecMatchItemList, kSecMatchSearchList};
            const void *values[] = {kSecClassCertificate, list, search};
            CFDictionaryRef query = CFDictionaryCreate(NULL, keys, values, 3, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
            status = SecItemDelete(query);
            if (status == errSecItemNotFound) status = errSecSuccess;
            CFRelease(query);
            CFRelease(list);
            CFRelease(search);
        }
    } else {
        // Use the signed-in user's keychain and trust domain, from the GUI process.
        // macOS owns the authentication UI; no password or authorization rules are changed.
        const void *keys[] = {kSecClass, kSecValueRef, kSecUseKeychain};
        const void *values[] = {kSecClassCertificate, certificate, keychain};
        CFDictionaryRef query = CFDictionaryCreate(NULL, keys, values, 3, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
        status = SecItemAdd(query, NULL);
        CFRelease(query);
        if (status == errSecSuccess || status == errSecDuplicateItem) {
            status = SecTrustSettingsSetTrustSettings(certificate, kSecTrustSettingsDomainUser, NULL);
        }
    }
    CFRelease(keychain);
    CFRelease(certificate);
    return status;
}
static char *mitmany_security_error(OSStatus status) {
    CFStringRef message = SecCopyErrorMessageString(status, NULL);
    if (message == NULL) return NULL;
    CFIndex capacity = CFStringGetMaximumSizeForEncoding(CFStringGetLength(message), kCFStringEncodingUTF8) + 1;
    char *text = malloc(capacity);
    if (text != NULL && !CFStringGetCString(message, text, capacity, kCFStringEncodingUTF8)) {
        free(text); text = NULL;
    }
    CFRelease(message);
    return text;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func nativeCertificate(data []byte, remove bool) error {
	var pointer unsafe.Pointer
	if len(data) > 0 {
		pointer = unsafe.Pointer(&data[0])
	}
	operation := C.int(0)
	if remove {
		operation = 1
	}
	status := C.mitmany_certificate(pointer, C.long(len(data)), operation)
	if status == 0 {
		return nil
	}
	message := "macOS Security error"
	if text := C.mitmany_security_error(status); text != nil {
		message = C.GoString(text)
		C.free(unsafe.Pointer(text))
	}
	if status == C.errSecUserCanceled {
		return fmt.Errorf("已取消系统证书授权，可重新点击安装")
	}
	return fmt.Errorf("macOS 证书授权失败（%d）：%s。可点击“打开钥匙串”完成信任设置，再重新检查", int(status), message)
}
