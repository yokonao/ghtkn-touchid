// The legacy file-based Keychain is the only one usable without the
// keychain-access-groups entitlement, and its APIs are all deprecated.
#pragma clang diagnostic ignored "-Wdeprecated-declarations"

#include <stdlib.h>
#include <string.h>
#include "keychain.h"

static CFStringRef string(const char *value) {
  return CFStringCreateWithCString(NULL, value, kCFStringEncodingUTF8);
}

static OSStatus query(const char *service, const char *account, CFMutableDictionaryRef *result) {
  SecKeychainRef keychain = NULL;
  OSStatus status = SecKeychainCopyDefault(&keychain);
  if (status != errSecSuccess) return status;

  CFStringRef serviceRef = string(service);
  CFStringRef accountRef = string(account);
  *result = CFDictionaryCreateMutable(NULL, 0, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
  CFDictionarySetValue(*result, kSecClass, kSecClassGenericPassword);
  CFDictionarySetValue(*result, kSecAttrService, serviceRef);
  CFDictionarySetValue(*result, kSecAttrAccount, accountRef);
  CFDictionarySetValue(*result, kSecUseKeychain, keychain);
  CFRelease(serviceRef);
  CFRelease(accountRef);
  CFRelease(keychain);
  return errSecSuccess;
}

// The item is readable only by this helper binary itself.
static OSStatus trusted_self(SecAccessRef *access) {
  SecTrustedApplicationRef application = NULL;
  OSStatus status = SecTrustedApplicationCreateFromPath(NULL, &application);
  if (status != errSecSuccess) return status;
  CFArrayRef applications = CFArrayCreate(NULL, (const void **)&application, 1, &kCFTypeArrayCallBacks);
  status = SecAccessCreate(CFSTR("ghtkn agent passphrase"), applications, access);
  CFRelease(applications);
  CFRelease(application);
  return status;
}

OSStatus kc_read(const char *service, const char *account, void **data, size_t *length) {
  CFMutableDictionaryRef q;
  OSStatus status = query(service, account, &q);
  if (status != errSecSuccess) return status;
  CFDictionarySetValue(q, kSecReturnData, kCFBooleanTrue);
  CFDictionarySetValue(q, kSecMatchLimit, kSecMatchLimitOne);

  CFTypeRef result = NULL;
  status = SecItemCopyMatching(q, &result);
  CFRelease(q);
  if (status != errSecSuccess) return status;

  *length = CFDataGetLength(result);
  *data = malloc(*length ? *length : 1);
  memcpy(*data, CFDataGetBytePtr(result), *length);
  CFRelease(result);
  return errSecSuccess;
}

OSStatus kc_upsert(const char *service, const char *account, const void *data, size_t length) {
  SecAccessRef access = NULL;
  OSStatus status = trusted_self(&access);
  if (status != errSecSuccess) return status;
  CFDataRef value = CFDataCreate(NULL, data, length);
  CFMutableDictionaryRef attributes = CFDictionaryCreateMutable(NULL, 0, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
  CFDictionarySetValue(attributes, kSecValueData, value);
  CFDictionarySetValue(attributes, kSecAttrAccess, access);
  CFRelease(value);
  CFRelease(access);

  CFMutableDictionaryRef q;
  status = query(service, account, &q);
  if (status == errSecSuccess) {
    status = SecItemUpdate(q, attributes);
    if (status == errSecItemNotFound) {
      CFIndex count = CFDictionaryGetCount(attributes);
      const void *keys[count], *values[count];
      CFDictionaryGetKeysAndValues(attributes, keys, values);
      for (CFIndex i = 0; i < count; i++) CFDictionarySetValue(q, keys[i], values[i]);
      status = SecItemAdd(q, NULL);
    }
    CFRelease(q);
  }
  CFRelease(attributes);
  return status;
}

OSStatus kc_delete(const char *service, const char *account) {
  CFMutableDictionaryRef q;
  OSStatus status = query(service, account, &q);
  if (status != errSecSuccess) return status;
  status = SecItemDelete(q);
  CFRelease(q);
  return status;
}

OSStatus kc_rename(const char *from, const char *to, const char *account) {
  CFMutableDictionaryRef q;
  OSStatus status = query(from, account, &q);
  if (status != errSecSuccess) return status;
  CFStringRef toRef = string(to);
  CFDictionaryRef attributes = CFDictionaryCreate(NULL, (const void **)&kSecAttrService, (const void **)&toRef, 1, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
  status = SecItemUpdate(q, attributes);
  CFRelease(attributes);
  CFRelease(toRef);
  CFRelease(q);
  return status;
}

char *kc_error(OSStatus status) {
  CFStringRef message = SecCopyErrorMessageString(status, NULL);
  if (!message) return NULL;
  CFIndex size = CFStringGetMaximumSizeForEncoding(CFStringGetLength(message), kCFStringEncodingUTF8) + 1;
  char *result = malloc(size);
  if (!CFStringGetCString(message, result, size, kCFStringEncodingUTF8)) result[0] = 0;
  CFRelease(message);
  return result;
}
