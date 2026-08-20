import Darwin
import Foundation
import LocalAuthentication
import Security

extension PassphraseStore {
  static func stage(_ passphrase: Data) throws -> LAContext {
    let context = try TouchID.authenticate(reason: "Reset the ghtkn agent passphrase")
    try delete(service: pendingService, context: context)
    try upsert(passphrase, service: pendingService, context: context)
    return context
  }

  static func commitPending(context: LAContext) throws {
    try delete(service: committedService, context: context)
    try rename(from: pendingService, to: committedService, context: context)

    do {
      try delete(service: activeService, context: context)
      try rename(from: committedService, to: activeService, context: context)
    } catch {
      writeStderr("ghtkn-touchid-reset: kept the committed passphrase for recovery: \(error)\n")
    }
  }

  #if TESTING
    static func cleanupForTesting() throws {
      let context = try TouchID.authenticate(reason: "Clean up test Keychain items")
      try delete(service: pendingService, context: context)
      try delete(service: committedService, context: context)
      try delete(service: activeService, context: context)
    }
  #endif

  private static func accessControl() throws -> SecAccessControl {
    var error: Unmanaged<CFError>?
    guard
      let access = SecAccessControlCreateWithFlags(
        nil,
        kSecAttrAccessibleWhenPasscodeSetThisDeviceOnly,
        .biometryCurrentSet,
        &error
      )
    else {
      throw HelperError(
        "create Keychain access control: \(error?.takeRetainedValue().localizedDescription ?? "unknown error")"
      )
    }
    return access
  }

  private static func upsert(_ passphrase: Data, service: String, context: LAContext) throws {
    let attributes: [String: Any] = [
      kSecValueData as String: passphrase,
      kSecAttrAccessControl as String: try accessControl(),
    ]
    let updateStatus = SecItemUpdate(
      query(service: service, context: context) as CFDictionary,
      attributes as CFDictionary
    )
    if updateStatus == errSecSuccess { return }
    guard updateStatus == errSecItemNotFound else {
      throw keychainError("update the passphrase in Keychain", updateStatus)
    }

    var item = query(service: service, context: context)
    item.merge(attributes) { _, new in new }
    let addStatus = SecItemAdd(item as CFDictionary, nil)
    guard addStatus == errSecSuccess else {
      throw keychainError("store the passphrase in Keychain", addStatus)
    }
  }

  private static func delete(service: String, context: LAContext) throws {
    let status = SecItemDelete(query(service: service, context: context) as CFDictionary)
    guard status == errSecSuccess || status == errSecItemNotFound else {
      throw keychainError("remove \(service) from Keychain", status)
    }
  }

  private static func rename(from: String, to: String, context: LAContext) throws {
    let status = SecItemUpdate(
      query(service: from, context: context) as CFDictionary,
      [kSecAttrService as String: to] as CFDictionary
    )
    guard status == errSecSuccess else {
      throw keychainError("commit the passphrase in Keychain", status)
    }
  }
}
