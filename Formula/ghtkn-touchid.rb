class GhtknTouchid < Formula
  desc "Unlock a local ghtkn agent with a Touch ID-protected passphrase"
  homepage "https://github.com/yokonao/ghtkn-touchid"
  url "https://github.com/yokonao/ghtkn-touchid/archive/refs/tags/v0.1.1.tar.gz"
  sha256 "b10edd7cf0d2f2a7af2b138d92043c99c926736af68dfb19ef333f2f9658be74"
  license "MIT"
  head "https://github.com/yokonao/ghtkn-touchid.git", branch: "main"

  depends_on :macos

  def install
    system "make", "build", "BUILD_DIR=build"
    bin.install "build/ghtkn-touchid", "build/ghtkn-touchid-reset"
  end

  def caveats
    <<~EOS
      Run ghtkn-touchid-reset once before the first unlock; it is what stores the
      passphrase in Keychain.

      The Keychain item trusts the installed binaries, so macOS asks for permission
      once after an upgrade replaces them.
    EOS
  end

  test do
    assert_match "usage: ghtkn-touchid", shell_output("#{bin}/ghtkn-touchid bogus 2>&1", 1)
  end
end
