import Darwin
import Foundation

private let newline = UInt8(ascii: "\n")

func runGhtknReset(passphrase: Data) throws -> Int32 {
  let ghtkn = try ghtknExecutable()
  let strings = [ghtkn, "agent", "reset"]
  var arguments: [UnsafeMutablePointer<CChar>?] = strings.map { strdup($0) }
  arguments.append(nil)
  defer {
    for pointer in arguments where pointer != nil {
      free(pointer)
    }
  }

  var master: Int32 = -1
  let pid = forkpty(&master, nil, nil, nil)
  guard pid >= 0 else {
    throw HelperError("start ghtkn: \(String(cString: strerror(errno)))")
  }
  if pid == 0 {
    _ = arguments.withUnsafeMutableBufferPointer { buffer in
      execv(buffer[0], buffer.baseAddress)
    }
    _exit(127)
  }

  defer { close(master) }
  do {
    try writePTY(master, Data("y\n".utf8))
    if let status = try waitForEchoDisabled(master, child: pid) {
      return exitCode(status)
    }

    var input = Data()
    input.reserveCapacity(passphrase.count * 2 + 2)
    input.append(passphrase)
    input.append(newline)
    input.append(passphrase)
    input.append(newline)
    defer { input.resetBytes(in: 0..<input.count) }
    try writePTY(master, input)
    try drainPTY(master)
  } catch {
    kill(pid, SIGTERM)
    // A session leader blocks in its exit path until the terminal output queue is
    // drained, so read the rest of it before waiting for the child.
    try? drainPTY(master)
    _ = try? waitForChild(pid)
    throw error
  }

  let status = try waitForChild(pid)
  return exitCode(status)
}

private func exitCode(_ status: Int32) -> Int32 {
  let signal = status & 0x7f
  return signal == 0 ? (status >> 8) & 0xff : 128 + signal
}

func ghtknExecutable() throws -> String {
  #if TESTING
    if let path = ProcessInfo.processInfo.environment["GHTKN_TOUCHID_TEST_GHTKN"] {
      return path
    }
  #endif

  let path = ProcessInfo.processInfo.environment["PATH"] ?? ""
  for directory in path.split(separator: ":", omittingEmptySubsequences: false) {
    guard directory.hasPrefix("/") else { continue }
    let candidate = URL(fileURLWithPath: String(directory), isDirectory: true)
      .appendingPathComponent("ghtkn").path
    var isDirectory: ObjCBool = false
    if FileManager.default.fileExists(atPath: candidate, isDirectory: &isDirectory),
      !isDirectory.boolValue,
      FileManager.default.isExecutableFile(atPath: candidate)
    {
      return candidate
    }
  }
  throw HelperError("ghtkn was not found in an absolute PATH entry")
}

private func waitForEchoDisabled(_ descriptor: Int32, child pid: pid_t) throws -> Int32? {
  for _ in 0..<200 {
    // The prompts written so far are discarded rather than left in the terminal
    // output queue, which would otherwise stall the child once it fills up.
    try drainReadable(descriptor)

    var settings = termios()
    if tcgetattr(descriptor, &settings) == 0, settings.c_lflag & tcflag_t(ECHO) == 0 {
      return nil
    }

    var status: Int32 = 0
    let result = waitpid(pid, &status, WNOHANG)
    if result == pid { return status }
    if result < 0, errno != EINTR {
      throw HelperError("wait for ghtkn: \(String(cString: strerror(errno)))")
    }
    usleep(10_000)
  }
  throw HelperError("ghtkn terminal echo remained enabled; refusing to send the passphrase")
}

private func writePTY(_ descriptor: Int32, _ data: Data) throws {
  try writeAll(data, operation: "write to ghtkn") { pointer, count in
    Darwin.write(descriptor, pointer, count)
  }
}

/// Reads and discards whatever the child has already written, without blocking.
private func drainReadable(_ descriptor: Int32) throws {
  while true {
    var descriptors = pollfd(fd: descriptor, events: Int16(POLLIN), revents: 0)
    let ready = poll(&descriptors, 1, 0)
    if ready < 0 {
      if errno == EINTR { continue }
      throw HelperError("poll ghtkn output: \(String(cString: strerror(errno)))")
    }
    if ready == 0 { return }

    var buffer = [UInt8](repeating: 0, count: 4096)
    defer {
      _ = buffer.withUnsafeMutableBytes {
        $0.initializeMemory(as: UInt8.self, repeating: 0)
      }
    }
    let count = Darwin.read(descriptor, &buffer, buffer.count)
    if count == 0 || count < 0 && errno == EIO { return }
    if count < 0 {
      if errno == EINTR { continue }
      throw HelperError("read ghtkn output: \(String(cString: strerror(errno)))")
    }
  }
}

private func drainPTY(_ descriptor: Int32) throws {
  while true {
    var buffer = [UInt8](repeating: 0, count: 4096)
    let count = Darwin.read(descriptor, &buffer, buffer.count)
    if count == 0 || count < 0 && errno == EIO { return }
    if count < 0 {
      if errno == EINTR { continue }
      throw HelperError("read ghtkn output: \(String(cString: strerror(errno)))")
    }
    _ = buffer.withUnsafeMutableBytes {
      $0.initializeMemory(as: UInt8.self, repeating: 0)
    }
  }
}

private func waitForChild(_ pid: pid_t) throws -> Int32 {
  var status: Int32 = 0
  while true {
    let result = waitpid(pid, &status, 0)
    if result == pid { return status }
    if result < 0, errno == EINTR { continue }
    throw HelperError("wait for ghtkn: \(String(cString: strerror(errno)))")
  }
}
