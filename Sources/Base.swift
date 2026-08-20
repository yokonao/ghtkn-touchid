import Foundation
import LocalAuthentication
import Security

struct HelperError: Error, CustomStringConvertible {
  let description: String

  init(_ description: String) {
    self.description = description
  }
}

func writeStderr(_ message: String) {
  FileHandle.standardError.write(Data(message.utf8))
}

func keychainError(_ operation: String, _ status: OSStatus) -> HelperError {
  let detail = SecCopyErrorMessageString(status, nil) as String? ?? "OSStatus \(status)"
  return HelperError("\(operation): \(detail)")
}

enum TouchID {
  static func authenticate(reason: String) throws -> LAContext {
    let context = LAContext()
    #if TESTING
      if ProcessInfo.processInfo.environment["GHTKN_TOUCHID_TESTING"] == "1" {
        return context
      }
    #endif

    context.localizedCancelTitle = "Cancel"
    context.localizedFallbackTitle = ""
    context.touchIDAuthenticationAllowableReuseDuration = 0

    var availabilityError: NSError?
    guard
      context.canEvaluatePolicy(.deviceOwnerAuthenticationWithBiometrics, error: &availabilityError)
    else {
      throw HelperError(availabilityError?.localizedDescription ?? "Touch ID is unavailable")
    }

    let semaphore = DispatchSemaphore(value: 0)
    var result = false
    var authenticationError: Error?
    context.evaluatePolicy(.deviceOwnerAuthenticationWithBiometrics, localizedReason: reason) {
      success, error in
      result = success
      authenticationError = error
      semaphore.signal()
    }
    semaphore.wait()

    guard result else {
      throw HelperError(
        authenticationError?.localizedDescription ?? "Touch ID authentication failed")
    }
    return context
  }
}

func writeAll(
  _ data: Data,
  operation: String,
  writer: (UnsafeRawPointer, Int) -> Int
) throws {
  try data.withUnsafeBytes { buffer in
    guard var pointer = buffer.baseAddress else { return }
    var remaining = buffer.count
    while remaining > 0 {
      let count = writer(pointer, remaining)
      if count < 0 {
        if errno == EINTR { continue }
        throw HelperError("\(operation): \(String(cString: strerror(errno)))")
      }
      guard count > 0 else {
        throw HelperError("\(operation): wrote zero bytes")
      }
      guard count <= remaining else {
        throw HelperError("\(operation): invalid byte count")
      }
      pointer = pointer.advanced(by: count)
      remaining -= count
    }
  }
}
