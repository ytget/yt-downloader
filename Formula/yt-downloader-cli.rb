# Deprecated: there is no supported headless CLI build of the Wails GUI app.
#
# Historical formula kept so taps that still reference it can surface a clear error.
# Install the desktop app via the GUI cask instead:
#   brew tap ytget/yt-downloader
#   brew install --cask yt-downloader
class YtDownloaderCli < Formula
  desc "DEPRECATED — use the yt-downloader cask for the desktop GUI"
  homepage "https://github.com/ytget/yt-downloader"
  url "https://github.com/ytget/yt-downloader/archive/refs/tags/v1.0.0.tar.gz"
  sha256 "0000000000000000000000000000000000000000000000000000000000000000"
  license "MIT"

  deprecate! date: "2026-08-02", because: "the project ships a Wails desktop GUI only; install the cask yt-downloader"

  def install
    odie <<~EOS
      yt-downloader-cli is deprecated.

      This project is a Wails desktop application (not a standalone CLI).
      Install the GUI instead:

        brew tap ytget/yt-downloader
        brew install --cask yt-downloader

      Or download a release from:
        https://github.com/ytget/yt-downloader/releases
    EOS
  end

  test do
    false
  end
end
