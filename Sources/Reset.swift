import Darwin
import Foundation
import Security

enum Reset {
  static func run() throws {
    guard isatty(STDIN_FILENO) == 1 else {
      throw HelperError("reset requires a terminal")
    }
    writeStderr("This deletes the ghtkn agent key and all cached tokens. Type RESET to continue: ")
    guard readLine() == "RESET" else {
      writeStderr("Canceled.\n")
      return
    }

    var passphrase = try randomPassphrase()
    defer { passphrase.resetBytes(in: 0..<passphrase.count) }
    try PassphraseStore.stage(passphrase)
    try performReset(
      run: { try runGhtknReset(passphrase: passphrase) },
      commit: { try PassphraseStore.commitPending() })
    writeStderr(
      "Reset the ghtkn agent with a generated passphrase. Start and unlock the agent.\n")
  }

  private static func randomPassphrase() throws -> Data {
    var random = [UInt8](repeating: 0, count: 32)
    defer {
      _ = random.withUnsafeMutableBytes {
        $0.initializeMemory(as: UInt8.self, repeating: 0)
      }
    }
    guard SecRandomCopyBytes(kSecRandomDefault, random.count, &random) == errSecSuccess else {
      throw HelperError("generate a random passphrase")
    }
    var bytes = Data(random)
    defer { bytes.resetBytes(in: 0..<bytes.count) }
    return bytes.base64EncodedData()
  }
}

func performReset(run: () throws -> Int32, commit: () throws -> Void) throws {
  let status: Int32
  do {
    status = try run()
  } catch {
    throw HelperError("ghtkn agent reset failed; kept the pending passphrase for recovery")
  }
  guard status == 0 else {
    throw HelperError("ghtkn agent reset exited \(status); kept the pending passphrase for recovery")
  }
  try commit()
}
