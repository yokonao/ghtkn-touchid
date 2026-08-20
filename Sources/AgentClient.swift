import Darwin
import Foundation

struct AgentResponse: Decodable {
  let ok: Bool
  let locked: Bool?
  let refreshTokenEnabled: Bool?
  let protocolVersion: Int?
  let minProtocolVersion: Int?
}

enum AgentClient {
  private static let protocolVersion = 1
  private static let refreshTokenTTL = 604_800_000_000_000

  static func unlock() throws {
    try unlock(loadCandidates: PassphraseStore.loadCandidates, sendRequest: send)
  }

  static func unlock(
    loadCandidates: () throws -> [StoredPassphrase],
    sendRequest: (AgentRequest) throws -> AgentResponse
  ) throws {
    let status = try sendRequest(.status)
    guard status.ok else {
      throw HelperError("query the ghtkn agent: request rejected")
    }
    try checkProtocol(status)
    if status.locked != true {
      writeStderr(
        "ghtkn agent is already unlocked; refresh_token_enabled=\(status.refreshTokenEnabled ?? false)\n"
      )
      return
    }

    var candidates = try loadCandidates()
    defer {
      for index in candidates.indices {
        candidates[index].data.resetBytes(in: 0..<candidates[index].data.count)
      }
    }

    var lastError = "unlock failed"
    for candidate in candidates {
      let response = try sendRequest(.unlock(candidate.data))
      if response.ok {
        writeStderr("ghtkn agent unlocked; refresh_token_enabled=true\n")
        return
      }
      lastError = "agent rejected the passphrase"
    }
    throw HelperError("unlock the ghtkn agent: \(lastError)")
  }

  private static func checkProtocol(_ response: AgentResponse) throws {
    let minimum = response.minProtocolVersion ?? 0
    guard
      let server = response.protocolVersion,
      minimum <= protocolVersion,
      protocolVersion <= server
    else {
      throw HelperError("unsupported ghtkn agent protocol")
    }
  }

  private static func socketPath() -> String {
    let environment = ProcessInfo.processInfo.environment
    if let path = environment["GHTKN_AGENT_SOCKET"], !path.isEmpty { return path }
    if let path = environment["XDG_RUNTIME_DIR"], !path.isEmpty {
      return URL(fileURLWithPath: path).appendingPathComponent("ghtkn/agent.sock").path
    }
    if let path = environment["XDG_CACHE_HOME"], !path.isEmpty {
      return URL(fileURLWithPath: path).appendingPathComponent("ghtkn/agent.sock").path
    }
    return FileManager.default.homeDirectoryForCurrentUser
      .appendingPathComponent(".cache/ghtkn/agent.sock").path
  }

  private static func send(_ request: AgentRequest) throws -> AgentResponse {
    var data = request.encoded()
    defer { data.resetBytes(in: 0..<data.count) }

    let descriptor = socket(AF_UNIX, SOCK_STREAM, 0)
    guard descriptor >= 0 else {
      throw HelperError("create the ghtkn agent socket: \(String(cString: strerror(errno)))")
    }
    defer { close(descriptor) }

    var enabled: Int32 = 1
    guard
      setsockopt(
        descriptor, SOL_SOCKET, SO_NOSIGPIPE, &enabled,
        socklen_t(MemoryLayout.size(ofValue: enabled))) == 0
    else {
      throw HelperError("configure the ghtkn agent socket: \(String(cString: strerror(errno)))")
    }
    var timeout = timeval(tv_sec: 10, tv_usec: 0)
    for option in [SO_RCVTIMEO, SO_SNDTIMEO] {
      guard
        setsockopt(
          descriptor, SOL_SOCKET, option, &timeout,
          socklen_t(MemoryLayout.size(ofValue: timeout))) == 0
      else {
        throw HelperError("configure the ghtkn agent socket: \(String(cString: strerror(errno)))")
      }
    }

    try connect(descriptor, path: socketPath())
    var peerUser = uid_t()
    var peerGroup = gid_t()
    guard getpeereid(descriptor, &peerUser, &peerGroup) == 0, peerUser == geteuid() else {
      throw HelperError("the ghtkn agent socket is not owned by the current user")
    }
    try writeAll(data, operation: "write to the ghtkn agent") { pointer, count in
      Darwin.write(descriptor, pointer, count)
    }
    var response = try readLine(descriptor)
    defer { response.resetBytes(in: 0..<response.count) }
    return try JSONDecoder.snakeCase.decode(AgentResponse.self, from: response)
  }

  static func connect(_ descriptor: Int32, path: String) throws {
    let pathBytes = Array(path.utf8CString)
    var address = sockaddr_un()
    guard pathBytes.count <= MemoryLayout.size(ofValue: address.sun_path) else {
      throw HelperError("ghtkn agent socket path is too long: \(path)")
    }
    address.sun_family = sa_family_t(AF_UNIX)
    let pathOffset = MemoryLayout<sockaddr_un>.offset(of: \.sun_path)!
    withUnsafeMutableBytes(of: &address) { addressBuffer in
      pathBytes.withUnsafeBytes { pathBuffer in
        addressBuffer.baseAddress!.advanced(by: pathOffset).copyMemory(
          from: pathBuffer.baseAddress!, byteCount: pathBuffer.count)
      }
    }

    let status = withUnsafePointer(to: &address) {
      $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
        Darwin.connect(descriptor, $0, socklen_t(MemoryLayout<sockaddr_un>.size))
      }
    }
    guard status == 0 else {
      throw HelperError("connect to the ghtkn agent: \(String(cString: strerror(errno)))")
    }
  }

  static func readLine(_ descriptor: Int32) throws -> Data {
    var response = Data()
    while response.count < 1_048_576 {
      var byte: UInt8 = 0
      let count = Darwin.read(descriptor, &byte, 1)
      if count < 0 {
        if errno == EINTR { continue }
        response.resetBytes(in: 0..<response.count)
        throw HelperError("read from the ghtkn agent: \(String(cString: strerror(errno)))")
      }
      if count == 0 || byte == 10 { return response }
      response.append(byte)
    }
    response.resetBytes(in: 0..<response.count)
    throw HelperError("the ghtkn agent response is too large")
  }
}

enum AgentRequest {
  case status
  case unlock(Data)

  func encoded() -> Data {
    switch self {
    case .status:
      return Data("{\"command\":\"STATUS\",\"protocol_version\":1}\n".utf8)
    case .unlock(let passphrase):
      var data = Data("{\"command\":\"UNLOCK\",\"protocol_version\":1,\"passphrase\":".utf8)
      data.appendJSONString(passphrase)
      data.append(
        Data(
          ",\"enable_refresh_token\":true,\"refresh_token_ttl\":604800000000000}\n".utf8))
      return data
    }
  }
}

extension Data {
  fileprivate mutating func appendJSONString(_ value: Data) {
    append(0x22)
    for byte in value {
      switch byte {
      case 0x22:
        append(contentsOf: [0x5c, 0x22])
      case 0x5c:
        append(contentsOf: [0x5c, 0x5c])
      case 0x08:
        append(contentsOf: [0x5c, 0x62])
      case 0x0c:
        append(contentsOf: [0x5c, 0x66])
      case 0x0a:
        append(contentsOf: [0x5c, 0x6e])
      case 0x0d:
        append(contentsOf: [0x5c, 0x72])
      case 0x09:
        append(contentsOf: [0x5c, 0x74])
      case 0x00...0x1f:
        let hex = Array("0123456789abcdef".utf8)
        append(contentsOf: [0x5c, 0x75, 0x30, 0x30, hex[Int(byte >> 4)], hex[Int(byte & 0x0f)]])
      default:
        append(byte)
      }
    }
    append(0x22)
  }
}

extension JSONDecoder {
  fileprivate static var snakeCase: JSONDecoder {
    let decoder = JSONDecoder()
    decoder.keyDecodingStrategy = .convertFromSnakeCase
    return decoder
  }
}
