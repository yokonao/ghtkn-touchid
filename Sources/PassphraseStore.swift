import Foundation
import LocalAuthentication
import Security

struct StoredPassphrase {
  let service: String
  var data: Data
}

enum PassphraseStore {
  static var baseService: String {
    #if TESTING
      ProcessInfo.processInfo.environment["GHTKN_TOUCHID_TEST_SERVICE"]
        ?? "ghtkn-touchid.agent-passphrase.test"
    #else
      "ghtkn-touchid.agent-passphrase"
    #endif
  }

  static var activeService: String { baseService }
  static var pendingService: String { "\(baseService).pending" }
  static var committedService: String { "\(baseService).committed" }
  static let account = NSUserName()

  static func loadCandidates() throws -> [StoredPassphrase] {
    let context = try TouchID.authenticate(reason: "Unlock the ghtkn agent")
    var candidates = [StoredPassphrase]()
    var firstError: Error?
    for service in [committedService, activeService, pendingService] {
      do {
        if let data = try read(service: service, context: context) {
          candidates.append(StoredPassphrase(service: service, data: data))
        }
      } catch {
        if firstError == nil { firstError = error }
      }
    }
    if candidates.isEmpty, let firstError { throw firstError }
    guard !candidates.isEmpty else {
      throw HelperError("the ghtkn passphrase is not stored in Keychain")
    }
    return candidates
  }

  static func query(service: String, context: LAContext? = nil) -> [String: Any] {
    var query: [String: Any] = [
      kSecClass as String: kSecClassGenericPassword,
      kSecAttrService as String: service,
      kSecAttrAccount as String: account,
      kSecUseDataProtectionKeychain as String: true,
    ]
    if let context {
      query[kSecUseAuthenticationContext as String] = context
    }
    return query
  }

  static func read(service: String, context: LAContext) throws -> Data? {
    var query = query(service: service, context: context)
    query[kSecReturnData as String] = true
    query[kSecMatchLimit as String] = kSecMatchLimitOne

    var result: CFTypeRef?
    let status = SecItemCopyMatching(query as CFDictionary, &result)
    if status == errSecItemNotFound { return nil }
    guard status == errSecSuccess else {
      throw keychainError("read the passphrase from Keychain", status)
    }
    guard let data = result as? Data, !data.isEmpty else {
      throw HelperError("the Keychain item contains no passphrase")
    }
    return data
  }
}
