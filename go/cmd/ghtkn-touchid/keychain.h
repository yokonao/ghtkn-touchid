#include <Security/Security.h>

OSStatus kc_read(const char *service, const char *account, void **data, size_t *length);
OSStatus kc_upsert(const char *service, const char *account, const void *data, size_t length);
OSStatus kc_delete(const char *service, const char *account);
OSStatus kc_rename(const char *from, const char *to, const char *account);
char *kc_error(OSStatus status);
