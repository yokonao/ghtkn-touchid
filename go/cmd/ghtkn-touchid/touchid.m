#import <LocalAuthentication/LocalAuthentication.h>
#include <string.h>
#include "touchid.h"

int touchid_authenticate(const char *reason, char **error) {
  @autoreleasepool {
    LAContext *context = [[LAContext alloc] init];
    context.localizedCancelTitle = @"Cancel";
    context.localizedFallbackTitle = @"";
    context.touchIDAuthenticationAllowableReuseDuration = 0;

    NSError *availability = nil;
    if (![context canEvaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics error:&availability]) {
      *error = strdup(availability.localizedDescription.UTF8String ?: "Touch ID is unavailable");
      [context release];
      return 0;
    }

    dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);
    __block BOOL success = NO;
    __block char *message = NULL;
    [context evaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics
            localizedReason:@(reason)
                      reply:^(BOOL ok, NSError *e) {
                        success = ok;
                        if (!ok) message = strdup(e.localizedDescription.UTF8String ?: "Touch ID authentication failed");
                        dispatch_semaphore_signal(semaphore);
                      }];
    dispatch_semaphore_wait(semaphore, DISPATCH_TIME_FOREVER);
    dispatch_release(semaphore);
    [context release];
    *error = message;
    return success;
  }
}
