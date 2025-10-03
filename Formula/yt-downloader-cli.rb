class YtDownloaderCli < Formula
  desc "Lightweight cross-platform desktop app to download YouTube videos and playlists"
  homepage "https://github.com/ytget/yt-downloader"
  url "https://github.com/ytget/yt-downloader/archive/v0.1.0.tar.gz"
  sha256 "08a308b5fefd50bc30c512f1fea195e551bf015479abf99d4f0ec236cbb3f149"
  license "MIT"
  head "https://github.com/ytget/yt-downloader.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", "-ldflags", "-X main.version=#{version}", "-o", bin/"yt-downloader", "main.go"
  end

  test do
    assert_match "yt-downloader", shell_output("#{bin}/yt-downloader --help", 1)
  end
end
