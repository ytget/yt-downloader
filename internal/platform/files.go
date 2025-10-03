package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Operating system constants
const (
	OSDarwin  = "darwin"
	OSWindows = "windows"
	OSLinux   = "linux"
	OSAndroid = "android"
)

// File permissions
const (
	DefaultDirPermissions = 0755
)

// Command constants
const (
	OpenCommand     = "open"
	ExplorerCommand = "explorer"
	XDGOpenCommand  = "xdg-open"
	CmdCommand      = "cmd"
	StartCommand    = "start"
)

// Command parameters
const (
	MacOSSelectFlag    = "-R"
	WindowsSelectParam = "/select,"
	WindowsCmdFlag     = "/c"
)

// File manager names
var (
	LinuxFileManagers = []string{"nautilus", "dolphin", "thunar", "nemo", "pcmanfm"}
)

// File length thresholds
const (
	MinFileNameLength    = 10
	MediumFileNameLength = 15
	LongFileNameLength   = 20
	MaxNameDifference    = 10
)

// Scoring system constants
const (
	ScoreForLongName     = 3
	ScoreForMediumName   = 2
	ScoreForShortName    = 1
	ScoreForSpaces       = 2
	ScoreForUnderscores  = 1
	ScoreForHyphens      = 1
	ScoreForVideoWords   = 2
	PenaltyForTimestamps = 1
)

// Video-related words for file detection
var (
	VideoRelatedWords = []string{"video", "music", "song", "track", "mix", "playlist", "album", "artist", "band", "rammstein"}
)

// Timestamp years for penalty
var (
	TimestampYears = []string{"2025", "2024"}
)

// File extensions to skip
var (
	SkippedExtensions = []string{".part", ".ytdl"}
)

// Common file name variations
var (
	FileNameVariations = []string{"-", "_", " "}
)

// OpenFileInManager opens the file in the system file manager and highlights it
func OpenFileInManager(filePath string) error {
	// Try to find the file with fallback to similar names
	foundPath, err := FindFileWithFallback(filePath)
	if err != nil {
		return fmt.Errorf("file does not exist: %v", err)
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(foundPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	switch runtime.GOOS {
	case OSDarwin: // macOS
		return openFileInFinderMacOS(absPath)
	case OSWindows:
		return openFileInExplorerWindows(absPath)
	case OSLinux:
		return openFileInManagerLinux(absPath)
	case OSAndroid:
		return openFileInManagerAndroid(absPath)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// openFileInFinderMacOS opens file in Finder on macOS with selection
func openFileInFinderMacOS(filePath string) error {
	cmd := exec.Command(OpenCommand, MacOSSelectFlag, filePath)
	return cmd.Run()
}

// openFileInExplorerWindows opens file in Explorer on Windows with selection
func openFileInExplorerWindows(filePath string) error {
	cmd := exec.Command(ExplorerCommand, WindowsSelectParam, filePath)
	return cmd.Run()
}

// openFileInManagerLinux opens directory containing file on Linux
// Note: File selection is not standardized on Linux, so we open the parent directory
func openFileInManagerLinux(filePath string) error {
	dir := filepath.Dir(filePath)

	// Try xdg-open first (most common)
	cmd := exec.Command(XDGOpenCommand, dir)
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Fallback to common file managers
	for _, fm := range LinuxFileManagers {
		if _, err := exec.LookPath(fm); err == nil {
			cmd := exec.Command(fm, dir)
			return cmd.Run()
		}
	}

	return fmt.Errorf("no suitable file manager found")
}

// openFileInManagerAndroid opens file in file manager on Android
func openFileInManagerAndroid(filePath string) error {

	var err error
	var cmd *exec.Cmd

	// Strategy 1: Try to open Downloads folder (most reliable)
	cmd = exec.Command("am", "start", "-a", "android.intent.action.VIEW", "-d", "content://com.android.externalstorage.documents/root/primary/Download")
	if err = cmd.Run(); err == nil {
		return nil
	}

	// Strategy 2: Try to open directory containing the file
	dir := filepath.Dir(filePath)
	cmd = exec.Command("am", "start", "-a", "android.intent.action.VIEW", "-d", "file://"+dir)
	if err = cmd.Run(); err == nil {
		return nil
	}

	// Strategy 3: Try to open with system Settings > Storage (always available)
	cmd = exec.Command("am", "start", "-a", "android.settings.INTERNAL_STORAGE_SETTINGS")
	if err = cmd.Run(); err == nil {
		return nil
	}

	// Strategy 4: Try to open with generic Settings
	cmd = exec.Command("am", "start", "-a", "android.settings.SETTINGS")
	if err = cmd.Run(); err == nil {
		return nil
	}

	// Strategy 5: Try to open file with system file picker
	cmd = exec.Command("am", "start", "-a", "android.intent.action.VIEW", "-d", "file://"+filePath)
	if err = cmd.Run(); err == nil {
		return nil
	}

	// Strategy 6: Try to open with specific file manager apps
	fileManagers := []string{
		"com.google.android.documentsui/.DocumentsActivity", // Files by Google
		"com.android.documentsui/.DocumentsActivity",        // System file manager
		"com.sec.android.app.myfiles/.MainActivity",         // Samsung My Files
		"com.mi.android.filemanager/.ui.MainActivity",       // MI File Manager
	}

	for _, fm := range fileManagers {
		cmd = exec.Command("am", "start", "-n", fm, "-d", "file://"+dir)
		if err = cmd.Run(); err == nil {
			return nil
		}
	}

	// Strategy 7: Try to share the file (fallback)
	cmd = exec.Command("am", "start", "-a", "android.intent.action.SEND", "-t", "video/*", "--eu", "android.intent.extra.STREAM", "file://"+filePath)
	if err = cmd.Run(); err == nil {
		return nil
	}

	return fmt.Errorf("failed to open file in manager: no suitable file manager found")
}

// CreateDirectoryIfNotExists creates directory if it doesn't exist
func CreateDirectoryIfNotExists(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return os.MkdirAll(dirPath, DefaultDirPermissions)
	}
	return nil
}

// OpenFileWithDefaultApp opens the file with the default system application
func OpenFileWithDefaultApp(filePath string) error {
	// Try to find the file with fallback to similar names
	foundPath, err := FindFileWithFallback(filePath)
	if err != nil {
		return fmt.Errorf("file does not exist: %v", err)
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(foundPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	switch runtime.GOOS {
	case OSDarwin: // macOS
		return openFileWithDefaultAppMacOS(absPath)
	case OSWindows:
		return openFileWithDefaultAppWindows(absPath)
	case OSLinux:
		return openFileWithDefaultAppLinux(absPath)
	case OSAndroid:
		return openFileWithDefaultAppAndroid(absPath)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// openFileWithDefaultAppMacOS opens file with default app on macOS
func openFileWithDefaultAppMacOS(filePath string) error {
	cmd := exec.Command(OpenCommand, filePath)
	return cmd.Run()
}

// openFileWithDefaultAppWindows opens file with default app on Windows
func openFileWithDefaultAppWindows(filePath string) error {
	cmd := exec.Command(CmdCommand, WindowsCmdFlag, StartCommand, "", filePath)
	return cmd.Run()
}

// openFileWithDefaultAppLinux opens file with default app on Linux
func openFileWithDefaultAppLinux(filePath string) error {
	// Try xdg-open first (most common)
	cmd := exec.Command(XDGOpenCommand, filePath)
	return cmd.Run()
}

// openFileWithDefaultAppAndroid opens file with default app on Android
func openFileWithDefaultAppAndroid(filePath string) error {

	// Professional approach: Use multiple strategies with proper error handling
	var err error
	var cmd *exec.Cmd

	// Strategy 1: Try with system Gallery app (most reliable for media files)
	cmd = exec.Command("am", "start", "-n", "com.android.gallery3d/.app.GalleryActivity", "-d", "file://"+filePath)
	if err = cmd.Run(); err == nil {
		return nil
	}

	// Strategy 2: Try with specific MIME type for MP4 files
	cmd = exec.Command("am", "start", "-a", "android.intent.action.VIEW", "-d", "file://"+filePath, "-t", "video/mp4")
	if err = cmd.Run(); err == nil {
		return nil
	}

	// Strategy 3: Try with generic video MIME type
	cmd = exec.Command("am", "start", "-a", "android.intent.action.VIEW", "-d", "file://"+filePath, "-t", "video/*")
	if err = cmd.Run(); err == nil {
		return nil
	}

	// Strategy 4: Try with audio MIME type
	cmd = exec.Command("am", "start", "-a", "android.intent.action.VIEW", "-d", "file://"+filePath, "-t", "audio/*")
	if err = cmd.Run(); err == nil {
		return nil
	}

	// Strategy 5: Try without MIME type (let system decide)
	cmd = exec.Command("am", "start", "-a", "android.intent.action.VIEW", "-d", "file://"+filePath)
	if err = cmd.Run(); err == nil {
		return nil
	}

	// Strategy 6: Try with content:// URI (modern Android)
	contentURI := "content://media/external/file" + filePath
	cmd = exec.Command("am", "start", "-a", "android.intent.action.VIEW", "-d", contentURI, "-t", "video/*")
	if err = cmd.Run(); err == nil {
		return nil
	}

	// Strategy 7: Try with VLC if available
	cmd = exec.Command("am", "start", "-n", "org.videolan.vlc/.gui.video.VideoPlayerActivity", "-d", "file://"+filePath)
	if err = cmd.Run(); err == nil {
		return nil
	}

	// Strategy 8: Try with MX Player if available
	cmd = exec.Command("am", "start", "-n", "com.mxtech.videoplayer.ad/.ActivityScreen", "-d", "file://"+filePath)
	if err = cmd.Run(); err == nil {
		return nil
	}

	// All strategies failed
	return fmt.Errorf("failed to open file with any method: no suitable app found")
}

// GetHomeDownloadsDir returns the standard Downloads directory for the user
func GetHomeDownloadsDir() (string, error) {
	// For Android, use the external storage Downloads directory
	// Check multiple ways to detect Android environment
	isAndroid := runtime.GOOS == "android" ||
		os.Getenv("ANDROID_DATA") != "" ||
		os.Getenv("ANDROID_ROOT") != "" ||
		os.Getenv("ANDROID_STORAGE") != "" ||
		filepath.Base(os.Args[0]) == "libdist.so" // Fyne Android apps run as libdist.so

	if isAndroid {
		// Use external storage Downloads directory so files appear in Gallery
		// This works with Scoped Storage on Android 11+ and is accessible via file manager
		downloadsDir := "/sdcard/Download"
		return downloadsDir, nil
	}

	// For other platforms, use the standard Downloads directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	downloadsDir := filepath.Join(homeDir, "Downloads")
	return downloadsDir, nil
}

// FindFileWithFallback tries to find a file by its original path, and if not found,
// searches for files with similar names in the same directory
func FindFileWithFallback(filePath string) (string, error) {
	// Validate input path
	if filePath == "" {
		return "", fmt.Errorf("file path is empty")
	}

	// Check if this looks like a URL instead of a file path
	if strings.HasPrefix(filePath, "http") {
		return "", fmt.Errorf("file path appears to be a URL: %s", filePath)
	}

	// Check if path contains proper separators
	if !strings.Contains(filePath, "/") && !strings.Contains(filePath, "\\") {
		return "", fmt.Errorf("file path does not contain path separators: %s", filePath)
	}

	// First, try the original path
	if _, err := os.Stat(filePath); err == nil {
		return filePath, nil
	}

	// If original path doesn't exist, try to find similar files
	dir := filepath.Dir(filePath)
	originalName := filepath.Base(filePath)
	originalExt := filepath.Ext(originalName)
	baseName := strings.TrimSuffix(originalName, originalExt)

	// Read directory contents
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	// Look for files with similar names
	var candidates []string
	var fallbackCandidates []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		entryName := entry.Name()
		entryExt := filepath.Ext(entryName)
		entryBase := strings.TrimSuffix(entryName, entryExt)

		// Check if this file could be our target
		if isSimilarFileName(entryBase, baseName) && entryExt == originalExt {
			candidates = append(candidates, filepath.Join(dir, entryName))
		}

		// Fallback: if no similar names found, look for files with same extension
		// that might be the downloaded file (e.g., video files with descriptive names)
		// Only consider files with the exact same extension
		if entryExt == originalExt && isLikelyDownloadedFile(entryName) {
			fallbackCandidates = append(fallbackCandidates, filepath.Join(dir, entryName))
		}
	}

	// If we found candidates with similar names, return the first one
	if len(candidates) > 0 {
		// Sort candidates to prefer exact matches first, then similar ones
		sort.Strings(candidates)
		return candidates[0], nil
	}

	// If no similar names found, try fallback candidates
	if len(fallbackCandidates) > 0 {
		// Sort by descriptive quality first, then by modification time
		sort.Slice(fallbackCandidates, func(i, j int) bool {
			fileNameI := filepath.Base(fallbackCandidates[i])
			fileNameJ := filepath.Base(fallbackCandidates[j])

			// Calculate descriptive score for each filename
			scoreI := getDescriptiveScore(fileNameI)
			scoreJ := getDescriptiveScore(fileNameJ)

			// If scores are significantly different, prefer higher score
			if scoreI != scoreJ {
				return scoreI > scoreJ
			}

			// If scores are similar, prefer more recent file
			infoI, _ := os.Stat(fallbackCandidates[i])
			infoJ, _ := os.Stat(fallbackCandidates[j])
			if infoI == nil || infoJ == nil {
				return false
			}
			return infoI.ModTime().After(infoJ.ModTime())
		})
		return fallbackCandidates[0], nil
	}

	return "", fmt.Errorf("file not found: %s", filePath)
}

// isSimilarFileName checks if two file names are similar enough to be considered the same file
func isSimilarFileName(name1, name2 string) bool {
	// Remove common prefixes/suffixes that might be added by downloaders
	clean1 := strings.TrimPrefix(strings.TrimSuffix(name1, " "), " ")
	clean2 := strings.TrimPrefix(strings.TrimSuffix(name2, " "), " ")

	// Check for exact match
	if clean1 == clean2 {
		return true
	}

	// Check for common variations
	variations := []string{
		"-" + clean1,
		clean1 + "-",
		"_" + clean1,
		clean1 + "_",
		" " + clean1,
		clean2 + " ",
	}

	for _, variation := range variations {
		if clean2 == variation {
			return true
		}
	}

	// Check if one is contained within the other (for truncated names)
	if strings.Contains(clean1, clean2) || strings.Contains(clean2, clean1) {
		// Only consider it similar if the difference is small
		diff := len(clean1) - len(clean2)
		if diff < 0 {
			diff = -diff
		}
		if diff <= MaxNameDifference { // Allow small differences
			return true
		}
	}

	return false
}

// isLikelyDownloadedFile checks if a filename looks like it could be a downloaded file
func isLikelyDownloadedFile(filename string) bool {
	// Skip temporary and metadata files
	for _, ext := range SkippedExtensions {
		if strings.HasSuffix(filename, ext) {
			return false
		}
	}

	// Skip files that are too short (likely not descriptive names)
	if len(filename) < MinFileNameLength {
		return false
	}

	// Look for patterns that suggest this is a downloaded video file
	// - Contains spaces (descriptive names)
	// - Contains underscores (descriptive names)
	// - Contains hyphens (descriptive names)
	// - Contains common video-related words
	hasSpaces := strings.Contains(filename, " ")
	hasUnderscores := strings.Contains(filename, "_")
	hasHyphens := strings.Contains(filename, "-")

	// Check for common video-related words
	hasVideoWords := false
	for _, word := range VideoRelatedWords {
		if strings.Contains(strings.ToLower(filename), word) {
			hasVideoWords = true
			break
		}
	}

	// File is likely downloaded if it has descriptive elements
	return hasSpaces || hasUnderscores || hasHyphens || hasVideoWords
}

// getDescriptiveScore calculates a score indicating how descriptive a filename is
func getDescriptiveScore(filename string) int {
	score := 0

	// Base score for length (longer names are usually more descriptive)
	if len(filename) > LongFileNameLength {
		score += ScoreForLongName
	} else if len(filename) > MediumFileNameLength {
		score += ScoreForMediumName
	} else if len(filename) > MinFileNameLength {
		score += ScoreForShortName
	}

	// Bonus for spaces (descriptive names)
	if strings.Contains(filename, " ") {
		score += ScoreForSpaces
	}

	// Bonus for underscores and hyphens
	if strings.Contains(filename, "_") {
		score += ScoreForUnderscores
	}
	if strings.Contains(filename, "-") {
		score += ScoreForHyphens
	}

	// Bonus for common video-related words
	for _, word := range VideoRelatedWords {
		if strings.Contains(strings.ToLower(filename), word) {
			score += ScoreForVideoWords
			break
		}
	}

	// Penalty for files that look like timestamps or random strings
	for _, year := range TimestampYears {
		if strings.Contains(filename, year) {
			score -= PenaltyForTimestamps
		}
	}

	return score
}

// NotifyMediaScanner notifies Android media scanner about new media files
// This makes downloaded videos appear in the Gallery app
func NotifyMediaScanner(filePath string) error {
	// Only for Android
	if runtime.GOOS != "android" && os.Getenv("ANDROID_DATA") == "" && os.Getenv("ANDROID_ROOT") == "" {
		return nil
	}

	// Use am broadcast to notify media scanner about the new file
	// This is the standard way to notify Android about new media files
	cmd := exec.Command("am", "broadcast", "-a", "android.intent.action.MEDIA_SCANNER_SCAN_FILE", "-d", "file://"+filePath)

	// Run the command in background, don't wait for it to complete
	// This prevents blocking the download process
	go func() {
		if err := cmd.Run(); err != nil {
			// Log error but don't fail the download
			fmt.Printf("Failed to notify media scanner about %s: %v\n", filePath, err)
		}
	}()

	return nil
}
