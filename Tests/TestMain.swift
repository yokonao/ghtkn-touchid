import Darwin
import Foundation
import Security

@main
enum TestMain {
  private static var assertions = 0

  static func main() throws {
    try testRequestEncoding()
    try testAlreadyUnlockedSkipsKeychain()
    try testProtocolMismatchSkipsKeychain()
    try testCandidateFallback()
    try testCandidateFallbackAndRedactedError()
    try testPartialAndInterruptedWrites()
    try testGhtknLookup()
    try testSocketBounds()
    try testResetRecoveryBoundary()
    try testPTY()
    print("ok: \(assertions) assertions")
  }

  private static func check(_ condition: @autoclosure () -> Bool, _ message: String) throws {
    assertions += 1
    if !condition() { throw HelperError("test failed: \(message)") }
  }

  private static func testRequestEncoding() throws {
    let secret = Data("pass\"\\\nphrase".utf8)
    let data = AgentRequest.unlock(secret).encoded()
    let object = try JSONSerialization.jsonObject(with: data) as? [String: Any]
    try check(object?["command"] as? String == "UNLOCK", "unlock command")
    try check(object?["passphrase"] as? String == "pass\"\\\nphrase", "JSON escaping")
    try check(object?["protocol_version"] as? Int == 1, "protocol version")
    try check(
      (object?["refresh_token_ttl"] as? NSNumber)?.int64Value == 604_800_000_000_000,
      "refresh TTL")
    let query = PassphraseStore.query(service: "test")
    try check(query[kSecUseDataProtectionKeychain as String] as? Bool == true, "data protection Keychain")
  }

  private static func testAlreadyUnlockedSkipsKeychain() throws {
    var loaded = false
    try AgentClient.unlock(
      loadCandidates: {
        loaded = true
        return []
      },
      sendRequest: { request in
        try check(request.encoded() == AgentRequest.status.encoded(), "status first")
        return response(ok: true, locked: false)
      })
    try check(!loaded, "already unlocked must skip Keychain")
  }

  private static func testProtocolMismatchSkipsKeychain() throws {
    var loaded = false
    do {
      try AgentClient.unlock(
        loadCandidates: {
          loaded = true
          return []
        },
        sendRequest: { _ in response(ok: true, locked: true, version: 2, minimum: 2) })
      throw HelperError("test failed: protocol mismatch succeeded")
    } catch let error as HelperError {
      try check(error.description == "unsupported ghtkn agent protocol", "protocol mismatch error")
    }
    try check(!loaded, "protocol mismatch must skip Keychain")
  }

  private static func testCandidateFallbackAndRedactedError() throws {
    let secret = "do-not-print-this-passphrase"
    var calls = 0
    do {
      try AgentClient.unlock(
        loadCandidates: {
          [StoredPassphrase(service: "active", data: Data(secret.utf8))]
        },
        sendRequest: { request in
          calls += 1
          return calls == 1 ? response(ok: true, locked: true) : response(ok: false, locked: nil)
        })
      throw HelperError("test failed: rejected passphrase succeeded")
    } catch let error as HelperError {
      try check(!error.description.contains(secret), "passphrase redaction")
      try check(error.description.contains("agent rejected"), "generic agent error")
    }
    try check(calls == 2, "status and unlock calls")
  }

  private static func testCandidateFallback() throws {
    var calls = 0
    var attempted = [String]()
    try AgentClient.unlock(
      loadCandidates: {
        [
          StoredPassphrase(service: "committed", data: Data("first".utf8)),
          StoredPassphrase(service: "active", data: Data("second".utf8)),
        ]
      },
      sendRequest: { request in
        calls += 1
        if case .unlock = request {
          let object = try JSONSerialization.jsonObject(with: request.encoded()) as? [String: Any]
          attempted.append(object?["passphrase"] as? String ?? "")
        }
        if calls == 1 { return response(ok: true, locked: true) }
        return response(ok: calls == 3, locked: nil)
      })
    try check(attempted == ["first", "second"], "recovery candidate order")
    try check(calls == 3, "candidate fallback")
  }

  private static func testPartialAndInterruptedWrites() throws {
    let input = Data("partial-write".utf8)
    var output = Data()
    var interrupted = true
    try writeAll(input, operation: "test write") { pointer, count in
      if interrupted {
        interrupted = false
        errno = EINTR
        return -1
      }
      let size = min(2, count)
      output.append(pointer.assumingMemoryBound(to: UInt8.self), count: size)
      return size
    }
    try check(output == input, "partial writes and EINTR")

    do {
      try writeAll(input, operation: "test write") { _, _ in 0 }
      throw HelperError("test failed: zero-byte write succeeded")
    } catch let error as HelperError {
      try check(error.description.contains("zero bytes"), "zero-byte write fails")
    }
  }

  private static func testResetRecoveryBoundary() throws {
    var committed = false
    do {
      try performReset(run: { 23 }, commit: { committed = true })
      throw HelperError("test failed: failed reset committed")
    } catch let error as HelperError {
      try check(error.description.contains("kept the pending passphrase"), "recovery message")
    }
    try check(!committed, "failed reset keeps pending state")

    do {
      try performReset(run: { throw HelperError("child setup") }, commit: { committed = true })
      throw HelperError("test failed: thrown reset committed")
    } catch let error as HelperError {
      try check(error.description.contains("kept the pending passphrase"), "thrown reset recovery")
    }

    try performReset(run: { 0 }, commit: { committed = true })
    try check(committed, "successful reset commits")
  }

  private static func testGhtknLookup() throws {
    let directory = FileManager.default.temporaryDirectory
      .appendingPathComponent("ghtkn-touchid-path-\(UUID().uuidString)")
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: directory) }
    let executable = directory.appendingPathComponent("ghtkn")
    try writeExecutable(executable, body: "exit 0")

    unsetenv("GHTKN_TOUCHID_TEST_GHTKN")
    let oldPath = ProcessInfo.processInfo.environment["PATH"]
    defer {
      if let oldPath {
        setenv("PATH", oldPath, 1)
      } else {
        unsetenv("PATH")
      }
    }

    setenv("PATH", ".:\(directory.path)", 1)
    let resolved = try ghtknExecutable()
    try check(resolved == executable.path, "absolute PATH lookup")
    setenv("PATH", ".:", 1)
    do {
      _ = try ghtknExecutable()
      throw HelperError("test failed: relative PATH entry accepted")
    } catch let error as HelperError {
      try check(error.description.contains("absolute PATH"), "relative PATH rejection")
    }
  }

  private static func testSocketBounds() throws {
    let descriptor = socket(AF_UNIX, SOCK_STREAM, 0)
    guard descriptor >= 0 else { throw HelperError("create test socket") }
    defer { close(descriptor) }
    do {
      try AgentClient.connect(descriptor, path: String(repeating: "a", count: 1_024))
      throw HelperError("test failed: oversized socket path accepted")
    } catch let error as HelperError {
      try check(error.description.contains("path is too long"), "socket path bound")
    }

    let response = FileManager.default.temporaryDirectory
      .appendingPathComponent("ghtkn-touchid-response-\(UUID().uuidString)")
    defer { try? FileManager.default.removeItem(at: response) }
    try Data(repeating: 0x61, count: 1_048_577).write(to: response)
    let file = open(response.path, O_RDONLY)
    guard file >= 0 else { throw HelperError("open test response") }
    defer { close(file) }
    do {
      _ = try AgentClient.readLine(file)
      throw HelperError("test failed: oversized response accepted")
    } catch let error as HelperError {
      try check(error.description.contains("response is too large"), "response bound")
    }
  }

  private static func testPTY() throws {
    let directory = FileManager.default.temporaryDirectory
      .appendingPathComponent("ghtkn-touchid-tests-\(UUID().uuidString)")
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: directory) }

    let success = directory.appendingPathComponent("success.sh")
    try writeExecutable(
      success,
      body: """
        test "$#" -eq 2 || exit 10
        test "$1" = agent && test "$2" = reset || exit 10
        printf 'confirmation text deliberately differs: '
        IFS= read -r answer
        test "$answer" = y || exit 11
        stty -echo
        printf 'first secret: '
        IFS= read -r first
        printf 'second secret: '
        IFS= read -r second
        stty echo
        test "$first" = "$second"
        """)
    setenv("GHTKN_TOUCHID_TEST_GHTKN", success.path, 1)
    let secret = Data("pty-only-secret".utf8)
    let status = try runGhtknReset(passphrase: secret)
    try check(status == 0, "PTY reset status \(status)")

    let echo = directory.appendingPathComponent("echo.sh")
    let received = directory.appendingPathComponent("received")
    try writeExecutable(
      echo,
      body: """
        printf 'confirmation: '
        IFS= read -r answer
        printf 'echo remains enabled: '
        IFS= read -r value
        printf '%s' "$value" > '\(received.path)'
        """)
    setenv("GHTKN_TOUCHID_TEST_GHTKN", echo.path, 1)
    do {
      _ = try runGhtknReset(passphrase: secret)
      throw HelperError("test failed: echo-enabled PTY accepted passphrase")
    } catch let error as HelperError {
      try check(error.description.contains("echo remained enabled"), "PTY echo guard")
    }
    try check(!FileManager.default.fileExists(atPath: received.path), "passphrase not sent with echo")

    let abnormal = directory.appendingPathComponent("abnormal.sh")
    try writeExecutable(
      abnormal,
      body: """
        printf 'confirmation: '
        IFS= read -r answer
        kill -TERM $$
        """)
    setenv("GHTKN_TOUCHID_TEST_GHTKN", abnormal.path, 1)
    let abnormalStatus = try runGhtknReset(passphrase: secret)
    try check(abnormalStatus == 143, "abnormal child exit")
  }

  private static func response(
    ok: Bool,
    locked: Bool?,
    version: Int = 1,
    minimum: Int = 0
  ) -> AgentResponse {
    AgentResponse(
      ok: ok,
      locked: locked,
      refreshTokenEnabled: false,
      protocolVersion: version,
      minProtocolVersion: minimum)
  }

  private static func writeExecutable(_ url: URL, body: String) throws {
    try Data("#!/bin/bash\n\(body)\n".utf8).write(to: url)
    guard chmod(url.path, 0o700) == 0 else {
      throw HelperError("chmod test executable: \(String(cString: strerror(errno)))")
    }
  }
}
