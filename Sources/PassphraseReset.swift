import Darwin
import Foundation
import Security

extension PassphraseStore {
  static func stage(_ passphrase: Data) throws {
    try TouchID.authenticate(reason: "Reset the ghtkn agent passphrase")
    try delete(service: pendingService)
    try upsert(passphrase, service: pendingService)
  }

  static func commitPending() throws {
    try delete(service: committedService)
    try rename(from: pendingService, to: committedService)

    do {
      try delete(service: activeService)
      try rename(from: committedService, to: activeService)
    } catch {
      writeStderr("ghtkn-touchid: kept the committed passphrase for recovery: \(error)\n")
    }
  }

  /// The Keychain item is readable only by this helper binary itself, so the
  /// passphrase never needs the `keychain-access-groups` entitlement that Touch ID
  /// protected items in the data protection Keychain require.
  private static func trustedHelper() throws -> SecAccess {
    var application: SecTrustedApplication?
    var status = SecTrustedApplicationCreateFromPath(nil, &application)
    guard status == errSecSuccess, let application else {
      throw keychainError("trust the helper", status)
    }

    var access: SecAccess?
    status = SecAccessCreate(
      "ghtkn agent passphrase" as CFString,
      [application] as CFArray,
      &access
    )
    guard status == errSecSuccess, let access else {
      throw keychainError("create Keychain access", status)
    }
    return access
  }

  private static func upsert(_ passphrase: Data, service: String) throws {
    let attributes: [String: Any] = [
      kSecValueData as String: passphrase,
      kSecAttrAccess as String: try trustedHelper(),
    ]
    let updateStatus = SecItemUpdate(
      try query(service: service) as CFDictionary,
      attributes as CFDictionary
    )
    if updateStatus == errSecSuccess { return }
    guard updateStatus == errSecItemNotFound else {
      throw keychainError("update the passphrase in Keychain", updateStatus)
    }

    var item = try query(service: service)
    item.merge(attributes) { _, new in new }
    let addStatus = SecItemAdd(item as CFDictionary, nil)
    guard addStatus == errSecSuccess else {
      throw keychainError("store the passphrase in Keychain", addStatus)
    }
  }

  private static func delete(service: String) throws {
    let status = SecItemDelete(try query(service: service) as CFDictionary)
    guard status == errSecSuccess || status == errSecItemNotFound else {
      throw keychainError("remove \(service) from Keychain", status)
    }
  }

  private static func rename(from: String, to: String) throws {
    let status = SecItemUpdate(
      try query(service: from) as CFDictionary,
      [kSecAttrService as String: to] as CFDictionary
    )
    guard status == errSecSuccess else {
      throw keychainError("commit the passphrase in Keychain", status)
    }
  }
}
