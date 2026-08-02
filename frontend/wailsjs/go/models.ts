export namespace config {
	
	export class SettingsData {
	    download_directory: string;
	    max_parallel_downloads: number;
	    quality_preset: string;
	    filename_template: string;
	    app_language: string;
	    auto_reveal_on_complete: boolean;
	    download_mode: string;
	    transcript_language: string;
	    collision_policy: string;
	    log_level: string;
	    log_to_file: boolean;
	    disable_captions: boolean;
	    video_only_safe_mode: boolean;
	    check_updates: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SettingsData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.download_directory = source["download_directory"];
	        this.max_parallel_downloads = source["max_parallel_downloads"];
	        this.quality_preset = source["quality_preset"];
	        this.filename_template = source["filename_template"];
	        this.app_language = source["app_language"];
	        this.auto_reveal_on_complete = source["auto_reveal_on_complete"];
	        this.download_mode = source["download_mode"];
	        this.transcript_language = source["transcript_language"];
	        this.collision_policy = source["collision_policy"];
	        this.log_level = source["log_level"];
	        this.log_to_file = source["log_to_file"];
	        this.disable_captions = source["disable_captions"];
	        this.video_only_safe_mode = source["video_only_safe_mode"];
	        this.check_updates = source["check_updates"];
	    }
	}

}

export namespace main {
	
	export class ActionResult {
	    ok: boolean;
	    message?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ActionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.message = source["message"];
	        this.error = source["error"];
	    }
	}
	export class AudioTrackDTO {
	    language_code: string;
	    display_name: string;
	    track_id?: string;
	    is_default: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AudioTrackDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language_code = source["language_code"];
	        this.display_name = source["display_name"];
	        this.track_id = source["track_id"];
	        this.is_default = source["is_default"];
	    }
	}
	export class DownloadRequest {
	    url: string;
	    mode?: string;
	    transcript_language?: string;
	    audio_language?: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.mode = source["mode"];
	        this.transcript_language = source["transcript_language"];
	        this.audio_language = source["audio_language"];
	    }
	}
	export class MediaProbeResult {
	    ok: boolean;
	    title?: string;
	    video_id?: string;
	    audio_tracks: AudioTrackDTO[];
	    error?: string;
	    needs_ffmpeg: boolean;
	    ffmpeg_found: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MediaProbeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.title = source["title"];
	        this.video_id = source["video_id"];
	        this.audio_tracks = this.convertValues(source["audio_tracks"], AudioTrackDTO);
	        this.error = source["error"];
	        this.needs_ffmpeg = source["needs_ffmpeg"];
	        this.ffmpeg_found = source["ffmpeg_found"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PlaylistVideoDTO {
	    id: string;
	    title: string;
	    duration: string;
	    url: string;
	    status: string;
	    progress: number;
	    error?: string;
	    output_path?: string;
	    file_size?: number;
	    speed?: string;
	    eta_sec?: number;
	    created_at?: string;
	    updated_at?: string;
	
	    static createFrom(source: any = {}) {
	        return new PlaylistVideoDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.duration = source["duration"];
	        this.url = source["url"];
	        this.status = source["status"];
	        this.progress = source["progress"];
	        this.error = source["error"];
	        this.output_path = source["output_path"];
	        this.file_size = source["file_size"];
	        this.speed = source["speed"];
	        this.eta_sec = source["eta_sec"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	    }
	}
	export class PlaylistDTO {
	    id: string;
	    title: string;
	    url: string;
	    videos: PlaylistVideoDTO[];
	    status: string;
	    total_videos: number;
	    downloaded: number;
	    error?: string;
	    created_at?: string;
	    updated_at?: string;
	
	    static createFrom(source: any = {}) {
	        return new PlaylistDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.url = source["url"];
	        this.videos = this.convertValues(source["videos"], PlaylistVideoDTO);
	        this.status = source["status"];
	        this.total_videos = source["total_videos"];
	        this.downloaded = source["downloaded"];
	        this.error = source["error"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class RestoreInfo {
	    pending: boolean;
	    total: number;
	    incomplete: number;
	
	    static createFrom(source: any = {}) {
	        return new RestoreInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pending = source["pending"];
	        this.total = source["total"];
	        this.incomplete = source["incomplete"];
	    }
	}
	export class TaskDTO {
	    id: string;
	    url: string;
	    status: string;
	    phase?: string;
	    progress: number;
	    percent: number;
	    speed: string;
	    eta_sec: number;
	    last_error?: string;
	    output_path?: string;
	    started_at?: string;
	    finished_at?: string;
	    title: string;
	    duration: string;
	    file_size: number;
	    mode: string;
	    transcript_status: string;
	    transcript_lang?: string;
	    transcript_srt_path?: string;
	    transcript_txt_path?: string;
	    transcript_error?: string;
	    warning?: string;
	    audio_language?: string;
	
	    static createFrom(source: any = {}) {
	        return new TaskDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.url = source["url"];
	        this.status = source["status"];
	        this.phase = source["phase"];
	        this.progress = source["progress"];
	        this.percent = source["percent"];
	        this.speed = source["speed"];
	        this.eta_sec = source["eta_sec"];
	        this.last_error = source["last_error"];
	        this.output_path = source["output_path"];
	        this.started_at = source["started_at"];
	        this.finished_at = source["finished_at"];
	        this.title = source["title"];
	        this.duration = source["duration"];
	        this.file_size = source["file_size"];
	        this.mode = source["mode"];
	        this.transcript_status = source["transcript_status"];
	        this.transcript_lang = source["transcript_lang"];
	        this.transcript_srt_path = source["transcript_srt_path"];
	        this.transcript_txt_path = source["transcript_txt_path"];
	        this.transcript_error = source["transcript_error"];
	        this.warning = source["warning"];
	        this.audio_language = source["audio_language"];
	    }
	}

}

export namespace updatecheck {
	
	export class Result {
	    checked: boolean;
	    update_available: boolean;
	    current: string;
	    latest?: string;
	    html_url?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.checked = source["checked"];
	        this.update_available = source["update_available"];
	        this.current = source["current"];
	        this.latest = source["latest"];
	        this.html_url = source["html_url"];
	        this.error = source["error"];
	    }
	}

}

