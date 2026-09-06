class GhtknTouchid < Formula
  desc "Unlock a local ghtkn agent with a Touch ID-protected passphrase"
  homepage "https://github.com/yokonao/ghtkn-touchid"
  url "https://github.com/yokonao/ghtkn-touchid/archive/refs/tags/v0.1.2.tar.gz"
  sha256 "20c6bbaddb05c7eeed62f9cf2674b552219e067e3f8a4236f10f5dec9c2ff4c9"
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
