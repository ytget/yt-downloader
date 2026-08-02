# GUI cask for the Wails desktop app.
#
# Maintained in-repo as the source of truth; publish/sync to the
# ytget/homebrew-yt-downloader tap on each GitHub Release.
#
# After uploading `yt-downloader.app.zip` to the release assets:
#   shasum -a 256 yt-downloader.app.zip
# and bump `version` + `sha256` below (or in the tap copy).
cask "yt-downloader" do
  version "1.0.0"
  sha256 :no_check # replace with release asset sha256 when v1.0.0 is published

  url "https://github.com/ytget/yt-downloader/releases/download/v#{version}/yt-downloader.app.zip"
  name "YT Downloader"
  desc "Desktop app to download YouTube videos, playlists, and captions"
  homepage "https://github.com/ytget/yt-downloader"

  livecheck do
    url :url
    strategy :github_latest
  end

  depends_on macos: ">= :monterey"

  app "yt-downloader.app"

  # Settings use os.UserConfigDir(): ~/Library/Application Support/yt-downloader/settings.json
  # Wails/webview may also leave caches under Caches.
  zap trash: [
    "~/Library/Application Support/yt-downloader",
    "~/Library/Caches/yt-downloader",
    "~/Library/Caches/com.wails.yt-downloader",
    "~/Library/Preferences/com.wails.yt-downloader.plist",
    "~/Library/Saved Application State/com.wails.yt-downloader.savedState",
  ]
end
