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
      writeStderr("ghtkn-touchid-reset: kept the committed passphrase for recovery: \(error)\n")
    }
  }

  #if TESTING
    static func cleanupForTesting() throws {
      try delete(service: pendingService)
      try delete(service: committedService)
      try delete(service: activeService)
    }
  #endif

  /// The Keychain item is readable only by the helper binaries themselves, so the
  /// passphrase never needs the `keychain-access-groups` entitlement that Touch ID
  /// protected items in the data protection Keychain require.
  private static func trustedHelpers() throws -> SecAccess {
    let paths: [String]
    #if TESTING
      if let value = ProcessInfo.processInfo.environment["GHTKN_TOUCHID_TEST_TRUSTED_APPS"] {
        paths = value.split(separator: ":").map(String.init)
      } else {
        paths = [CommandLine.arguments[0]]
      }
    #else
      let directory = try helperDirectory()
      paths = ["ghtkn-touchid", "ghtkn-touchid-reset"].map {
        directory.appendingPathComponent($0).path
      }
    #endif

    var applications = [SecTrustedApplication]()
    for path in paths {
      var application: SecTrustedApplication?
      let status = path.withCString {
        SecTrustedApplicationCreateFromPath($0, &application)
      }
      guard status == errSecSuccess, let application else {
        throw keychainError("trust \(path)", status)
      }
      applications.append(application)
    }

    var access: SecAccess?
    let status = SecAccessCreate(
      "ghtkn agent passphrase" as CFString,
      applications as CFArray,
      &access
    )
    guard status == errSecSuccess, let access else {
      throw keychainError("create Keychain access", status)
    }
    return access
  }

  /// Both helpers are installed side by side, so the unlock helper is trusted from
  /// the directory this reset helper runs from rather than a fixed install prefix.
  private static func helperDirectory() throws -> URL {
    guard let executable = Bundle.main.executableURL ?? URL(string: CommandLine.arguments[0]) else {
      throw HelperError("locate the helper directory")
    }
    return executable.resolvingSymlinksInPath().deletingLastPathComponent()
  }

  private static func upsert(_ passphrase: Data, service: String) throws {
    let attributes: [String: Any] = [
      kSecValueData as String: passphrase,
      kSecAttrAccess as String: try trustedHelpers(),
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
