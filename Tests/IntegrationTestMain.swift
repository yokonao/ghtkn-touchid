import Foundation

@main
enum IntegrationTestMain {
  static func main() {
    do {
      try run()
      print("ok: ghtkn integration test passed")
    } catch {
      writeStderr("ghtkn-touchid-integration: \(error)\n")
      exit(1)
    }
  }

  private static func run() throws {
    let ghtkn = try ghtknExecutable()

    let root = FileManager.default.temporaryDirectory
      .appendingPathComponent("ghtkn-touchid-integration-\(UUID().uuidString)")
    try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: root) }

    setenv("GHTKN_AGENT_SOCKET", root.appendingPathComponent("agent.sock").path, 1)
    setenv("GHTKN_AGENT_KEY", root.appendingPathComponent("key").path, 1)
    setenv("GHTKN_AGENT_TOKEN_DIR", root.appendingPathComponent("tokens").path, 1)

    let passphrase = Data("ghtkn-touchid-integration-\(UUID().uuidString)".utf8)
    let resetStatus = try runGhtknReset(passphrase: passphrase)
    guard resetStatus == 0 else {
      throw HelperError("ghtkn agent reset exited \(resetStatus)")
    }

    let agent = Process()
    agent.executableURL = URL(fileURLWithPath: ghtkn)
    agent.arguments = ["agent", "start"]
    let output = Pipe()
    agent.standardOutput = output
    agent.standardError = output
    try agent.run()
    defer {
      agent.terminate()
      agent.waitUntilExit()
    }

    try waitForSocket(root.appendingPathComponent("agent.sock").path, process: agent, output: output)

    var status = try AgentClient.send(.status)
    guard status.ok, status.locked == true else {
      throw HelperError("a freshly reset ghtkn agent was not locked")
    }

    try AgentClient.unlock(
      loadCandidates: { [StoredPassphrase(service: "integration", data: passphrase)] },
      sendRequest: AgentClient.send)

    status = try AgentClient.send(.status)
    guard status.ok, status.locked != true else {
      throw HelperError("the ghtkn agent was still locked after unlock")
    }
  }

  private static func waitForSocket(_ path: String, process: Process, output: Pipe) throws {
    for _ in 0..<100 {
      if FileManager.default.fileExists(atPath: path) { return }
      if !process.isRunning {
        let data = output.fileHandleForReading.availableData
        throw HelperError(
          "ghtkn agent start exited early: \(String(data: data, encoding: .utf8) ?? "")")
      }
      usleep(50_000)
    }
    throw HelperError("the ghtkn agent socket never appeared at \(path)")
  }
}
