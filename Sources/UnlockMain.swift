import Foundation

@main
enum UnlockMain {
  static func main() {
    do {
      let arguments = Array(CommandLine.arguments.dropFirst())
      guard arguments.isEmpty || arguments == ["unlock"] else {
        throw HelperError("usage: ghtkn-touchid [unlock]")
      }
      try AgentClient.unlock()
    } catch {
      writeStderr("ghtkn-touchid: \(error)\n")
      exit(1)
    }
  }
}
