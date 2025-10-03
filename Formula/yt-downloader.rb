cask "yt-downloader" do
  version "0.1.0"
  sha256 "e9c2086d4b66631d0e8515447a28498ef81d43ef641c36d4f002f136864ed179"

  url "https://github.com/ytget/yt-downloader/releases/download/v#{version}/yt-downloader.app.zip"
  name "YT Downloader"
  desc "Lightweight cross-platform desktop app to download YouTube videos and playlists"
  homepage "https://github.com/ytget/yt-downloader"

  livecheck do
    url :stable
    regex(/^v?(\d+(?:\.\d+)+)$/i)
  end

  app "yt-downloader.app"

  zap trash: [
    "~/Library/Application Support/yt-downloader",
    "~/Library/Preferences/com.github.ytget.ytdownloader.plist",
    "~/Library/Saved Application State/com.github.ytget.ytdownloader.savedState",
  ]
end
