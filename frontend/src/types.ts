export type TaskStatus =
  | 'Pending'
  | 'Starting'
  | 'Downloading'
  | 'Stopping'
  | 'Paused'
  | 'Stopped'
  | 'Completed'
  | 'Error'

export type TaskPhase =
  | 'queued'
  | 'resolving'
  | 'downloading'
  | 'captions'
  | 'writing'
  | 'done'
  | string

export type DownloadMode = 'video' | 'video_and_transcript' | 'transcript_only' | string
export type TranscriptStatus = 'none' | 'pending' | 'completed' | 'unavailable' | 'error' | string

export interface DownloadTask {
  id: string
  url: string
  status: TaskStatus
  phase?: TaskPhase
  progress: number
  percent: number
  speed: string
  eta_sec: number
  last_error?: string
  output_path?: string
  started_at?: string
  finished_at?: string
  title: string
  duration: string
  file_size: number
  mode?: DownloadMode
  transcript_status?: TranscriptStatus
  transcript_lang?: string
  transcript_srt_path?: string
  transcript_txt_path?: string
  transcript_error?: string
  warning?: string
  audio_language?: string
}

export interface DownloadRequest {
  url: string
  mode?: DownloadMode
  transcript_language?: string
  audio_language?: string
}

export interface AudioTrack {
  language_code: string
  display_name: string
  track_id?: string
  is_default?: boolean
}

export interface MediaProbeResult {
  ok: boolean
  title?: string
  video_id?: string
  audio_tracks?: AudioTrack[]
  error?: string
  needs_ffmpeg?: boolean
  ffmpeg_found?: boolean
}

export interface PlaylistVideo {
  id: string
  title: string
  duration: string
  url: string
  status: string
  progress: number
  error?: string
  output_path?: string
  file_size?: number
  speed?: string
  eta_sec?: number
}

export interface Playlist {
  id: string
  title: string
  url: string
  videos: PlaylistVideo[]
  status: string
  total_videos: number
  downloaded: number
  error?: string
}

export type CollisionPolicy = 'skip' | 'overwrite' | 'rename' | string

export interface SettingsData {
  download_directory: string
  max_parallel_downloads: number
  quality_preset: 'best' | 'medium' | 'audio' | string
  filename_template: string
  app_language: string
  auto_reveal_on_complete: boolean
  download_mode: DownloadMode
  transcript_language: string
  collision_policy?: CollisionPolicy
  log_level?: string
  log_to_file?: boolean
  disable_captions?: boolean
  video_only_safe_mode?: boolean
  check_updates?: boolean
}

export interface UpdateCheckResult {
  checked?: boolean
  update_available?: boolean
  current?: string
  latest?: string
  html_url?: string
  error?: string
}

export interface ActionResult {
  ok: boolean
  message?: string
  error?: string
}

export interface NotificationPayload {
  message: string
  spinning?: boolean
  level?: 'info' | 'error' | 'success' | 'warn' | string
}

export interface QueueRestorePayload {
  total: number
  incomplete: number
}

export type StatusFilter = 'all' | 'downloading' | 'pending' | 'completed' | 'errors'
