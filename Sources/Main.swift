import Foundation

@main
enum Main {
  static func main() {
    do {
      switch Array(CommandLine.arguments.dropFirst()) {
      case [], ["unlock"]:
        try AgentClient.unlock()
      case ["reset"]:
        try Reset.run()
      default:
        throw HelperError("usage: ghtkn-touchid [unlock|reset]")
      }
    } catch {
      writeStderr("ghtkn-touchid: \(error)\n")
      exit(1)
    }
  }
}
