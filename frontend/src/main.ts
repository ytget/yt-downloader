import './style.css'
import { api, eventsOn } from './api'
import { setLang, t } from './i18n'
import type {
  ActionResult,
  AudioTrack,
  DownloadMode,
  DownloadTask,
  NotificationPayload,
  QueueRestorePayload,
  SettingsData,
  StatusFilter,
  TaskStatus,
} from './types'

const state = {
  version: 'dev',
  tasks: [] as DownloadTask[],
  filter: 'all' as StatusFilter,
  settings: null as SettingsData | null,
  busy: false,
  mode: 'video' as DownloadMode,
  transcriptLang: 'auto',
  shellReady: false,
  restore: null as QueueRestorePayload | null,
  qualityHelp: {} as Record<string, string>,
  pendingURL: '' as string,
  audioTracks: [] as AudioTrack[],
  audioPickTitle: '' as string,
  audioFFmpegMissing: false,
}

const appEl = document.querySelector<HTMLDivElement>('#app')!

function formatETA(sec: number): string {
  if (!sec || sec <= 0) return '—'
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = sec % 60
  if (h > 0) return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

function statusClass(status: TaskStatus | string): string {
  return String(status || '').toLowerCase()
}

function matchesFilter(task: DownloadTask, filter: StatusFilter): boolean {
  switch (filter) {
    case 'downloading':
      return task.status === 'Downloading' || task.status === 'Starting' || task.status === 'Stopping'
    case 'pending':
      return task.status === 'Pending' || task.status === 'Paused'
    case 'completed':
      return task.status === 'Completed'
    case 'errors':
      return task.status === 'Error'
    default:
      return true
  }
}

function showNotice(message: string, level: string = 'info', spinning = false) {
  const el = document.querySelector<HTMLDivElement>('#notice')
  if (!el) return
  el.className = `notice visible ${level}`
  el.innerHTML = `${spinning ? '<span class="spinner"></span>' : ''}<span>${escapeHtml(message)}</span>`
}

async function runAction(
  action: () => Promise<ActionResult>,
  successMessage: string,
  successLevel: 'info' | 'success' = 'info',
): Promise<void> {
  const result = await action()
  if (!result.ok) showNotice(result.error || 'Failed', 'error')
  else showNotice(result.message || successMessage, successLevel)
  await refreshAndRenderTasks()
}

function hideNoticeSoon() {
  window.setTimeout(() => {
    const el = document.querySelector<HTMLDivElement>('#notice')
    if (el) el.classList.remove('visible')
  }, 4000)
}

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

function phaseLabel(phase?: string): string {
  switch (phase) {
    case 'resolving':
      return t('phase_resolving')
    case 'downloading':
      return t('phase_downloading')
    case 'captions':
      return t('phase_captions')
    case 'writing':
      return t('phase_writing')
    case 'done':
      return t('phase_done')
    case 'queued':
      return t('phase_queued')
    default:
      return ''
  }
}

function filteredTasks(): DownloadTask[] {
  return state.tasks.filter((task) => matchesFilter(task, state.filter))
}

/** Full shell render (structure). Task rows updated incrementally afterward. */
function renderShell(): void {
  const scrollTop = document.querySelector('#task-list')?.scrollTop ?? 0
  const quality = state.settings?.quality_preset || 'medium'

  appEl.innerHTML = `
    <div class="app-shell">
      <header class="header">
        <div class="brand">
          <div class="logo">YT</div>
          <div>
            <h1>${escapeHtml(t('app_title'))}</h1>
            <div class="version">v${escapeHtml(state.version)}</div>
          </div>
        </div>
        <button class="btn btn-ghost" id="settings-btn" type="button">${escapeHtml(t('settings'))}</button>
      </header>

      <section class="card">
        <form class="input-row" id="download-form">
          <input id="url-input" type="text" placeholder="${escapeHtml(t('enter_url'))}" autocomplete="off" />
          <button class="btn btn-ghost" id="paste-url-btn" type="button" title="${escapeHtml(t('paste'))}">${escapeHtml(t('paste'))}</button>
          <button class="btn btn-primary" id="download-btn" type="submit" ${state.busy ? 'disabled' : ''}>
            ${escapeHtml(t('download'))}
          </button>
        </form>
        <div class="mode-row">
          <span class="mode-label">${escapeHtml(t('mode'))}</span>
          <div class="filters" id="mode-filters">
            ${
              (
                [
                  ['video', 'mode_video'],
                  ['video_and_transcript', 'mode_video_transcript'],
                  ['transcript_only', 'mode_transcript'],
                ] as Array<[DownloadMode, string]>
              )
                .map(
                  ([mode, key]) =>
                    `<button type="button" class="chip ${state.mode === mode ? 'active' : ''}" data-mode="${mode}">${escapeHtml(
                      t(key),
                    )}</button>`,
                )
                .join('')
            }
          </div>
          <label class="mode-label" for="transcript-lang">${escapeHtml(t('transcript_lang'))}</label>
          <select id="transcript-lang" class="lang-select">
            <option value="auto" ${state.transcriptLang === 'auto' ? 'selected' : ''}>${escapeHtml(t('lang_auto'))}</option>
            <option value="en" ${state.transcriptLang === 'en' ? 'selected' : ''}>English</option>
            <option value="ru" ${state.transcriptLang === 'ru' ? 'selected' : ''}>Русский</option>
            <option value="pt" ${state.transcriptLang === 'pt' ? 'selected' : ''}>Português</option>
            <option value="es" ${state.transcriptLang === 'es' ? 'selected' : ''}>Español</option>
            <option value="de" ${state.transcriptLang === 'de' ? 'selected' : ''}>Deutsch</option>
            <option value="fr" ${state.transcriptLang === 'fr' ? 'selected' : ''}>Français</option>
          </select>
        </div>
        <div id="notice" class="notice"></div>
        <div id="restore-banner" class="restore-banner ${state.restore ? 'visible' : ''}"></div>
      </section>

      <div class="toolbar">
        <div class="filters" id="filters">
          ${(['all', 'downloading', 'pending', 'completed', 'errors'] as StatusFilter[])
            .map(
              (f) =>
                `<button type="button" class="chip ${state.filter === f ? 'active' : ''}" data-filter="${f}">${escapeHtml(
                  t(f),
                )}</button>`,
            )
            .join('')}
        </div>
        <div class="batch-actions">
          <button class="btn btn-ghost btn-sm" id="pause-all" type="button">${escapeHtml(t('pause_all'))}</button>
          <button class="btn btn-ghost btn-sm" id="retry-failed" type="button">${escapeHtml(t('retry_failed'))}</button>
          <button class="btn btn-ghost btn-sm" id="clear-completed" type="button">${escapeHtml(t('clear_completed'))}</button>
          <span class="version" id="task-count">0 / 0</span>
        </div>
      </div>

      <section class="card list" id="task-list"></section>
    </div>

    <div class="modal-backdrop" id="settings-modal">
      <div class="modal">
        <h2>${escapeHtml(t('settings'))}</h2>
        <div class="form-grid">
          <div class="field">
            <label>${escapeHtml(t('download_directory'))}</label>
            <div class="field-row">
              <input id="setting-dir" type="text" value="${escapeHtml(state.settings?.download_directory || '')}" />
              <button class="btn btn-ghost" id="browse-dir" type="button">${escapeHtml(t('browse'))}</button>
              <button class="btn btn-ghost" id="open-dir" type="button">${escapeHtml(t('open_folder'))}</button>
            </div>
          </div>
          <div class="field">
            <label>${escapeHtml(t('mode'))}</label>
            <select id="setting-mode">
              <option value="video" ${state.settings?.download_mode === 'video' ? 'selected' : ''}>${escapeHtml(t('mode_video'))}</option>
              <option value="video_and_transcript" ${state.settings?.download_mode === 'video_and_transcript' ? 'selected' : ''}>${escapeHtml(t('mode_video_transcript'))}</option>
              <option value="transcript_only" ${state.settings?.download_mode === 'transcript_only' ? 'selected' : ''}>${escapeHtml(t('mode_transcript'))}</option>
            </select>
          </div>
          <div class="field">
            <label>${escapeHtml(t('quality_preset'))}</label>
            <select id="setting-quality">
              <option value="best" ${quality === 'best' ? 'selected' : ''}>${escapeHtml(t('best'))}</option>
              <option value="medium" ${quality === 'medium' || quality === 'audio' ? 'selected' : ''}>${escapeHtml(t('medium'))}</option>
              <option value="audio" disabled>${escapeHtml(t('audio_disabled'))}</option>
            </select>
            <div class="field-hint" id="quality-hint">${escapeHtml(qualityHint(quality === 'audio' ? 'medium' : quality))}</div>
          </div>
          <div class="field">
            <label>${escapeHtml(t('max_parallel'))}</label>
            <input id="setting-parallel" type="number" min="1" max="10" value="${state.settings?.max_parallel_downloads ?? 2}" />
          </div>
          <div class="field">
            <label>${escapeHtml(t('filename_template'))}</label>
            <input id="setting-template" type="text" value="${escapeHtml(state.settings?.filename_template || '%(title)s.%(ext)s')}" />
          </div>
          <div class="field">
            <label>${escapeHtml(t('collision_policy'))}</label>
            <select id="setting-collision">
              <option value="rename" ${(!state.settings?.collision_policy || state.settings?.collision_policy === 'rename') ? 'selected' : ''}>${escapeHtml(t('collision_rename'))}</option>
              <option value="skip" ${state.settings?.collision_policy === 'skip' ? 'selected' : ''}>${escapeHtml(t('collision_skip'))}</option>
              <option value="overwrite" ${state.settings?.collision_policy === 'overwrite' ? 'selected' : ''}>${escapeHtml(t('collision_overwrite'))}</option>
            </select>
          </div>
          <div class="field">
            <label>${escapeHtml(t('language'))}</label>
            <select id="setting-lang">
              <option value="system" ${state.settings?.app_language === 'system' ? 'selected' : ''}>System</option>
              <option value="lt" ${!state.settings?.app_language || state.settings?.app_language === 'lt' ? 'selected' : ''}>Lietuvių</option>
              <option value="en" ${state.settings?.app_language === 'en' ? 'selected' : ''}>English</option>
              <option value="ru" ${state.settings?.app_language === 'ru' ? 'selected' : ''}>Русский</option>
              <option value="pt" ${state.settings?.app_language === 'pt' ? 'selected' : ''}>Português</option>
            </select>
          </div>
          <div class="field">
            <label>${escapeHtml(t('transcript_lang'))}</label>
            <select id="setting-transcript-lang">
              <option value="auto" ${(!state.settings?.transcript_language || state.settings?.transcript_language === 'auto') ? 'selected' : ''}>${escapeHtml(t('lang_auto'))}</option>
              <option value="en" ${state.settings?.transcript_language === 'en' ? 'selected' : ''}>English</option>
              <option value="ru" ${state.settings?.transcript_language === 'ru' ? 'selected' : ''}>Русский</option>
              <option value="pt" ${state.settings?.transcript_language === 'pt' ? 'selected' : ''}>Português</option>
              <option value="es" ${state.settings?.transcript_language === 'es' ? 'selected' : ''}>Español</option>
              <option value="de" ${state.settings?.transcript_language === 'de' ? 'selected' : ''}>Deutsch</option>
              <option value="fr" ${state.settings?.transcript_language === 'fr' ? 'selected' : ''}>Français</option>
            </select>
          </div>
          <label class="check">
            <input id="setting-reveal" type="checkbox" ${state.settings?.auto_reveal_on_complete ? 'checked' : ''} />
            <span>${escapeHtml(t('auto_reveal'))}</span>
          </label>
          <div class="field">
            <label>${escapeHtml(t('log_level'))}</label>
            <select id="setting-log-level">
              <option value="error" ${state.settings?.log_level === 'error' ? 'selected' : ''}>error</option>
              <option value="warn" ${state.settings?.log_level === 'warn' ? 'selected' : ''}>warn</option>
              <option value="info" ${(!state.settings?.log_level || state.settings?.log_level === 'info') ? 'selected' : ''}>info</option>
              <option value="debug" ${state.settings?.log_level === 'debug' ? 'selected' : ''}>debug</option>
            </select>
          </div>
          <label class="check">
            <input id="setting-log-file" type="checkbox" ${state.settings?.log_to_file !== false ? 'checked' : ''} />
            <span>${escapeHtml(t('log_to_file'))}</span>
          </label>
          <label class="check">
            <input id="setting-disable-captions" type="checkbox" ${state.settings?.disable_captions ? 'checked' : ''} />
            <span>${escapeHtml(t('disable_captions'))}</span>
          </label>
          <label class="check">
            <input id="setting-video-only" type="checkbox" ${state.settings?.video_only_safe_mode ? 'checked' : ''} />
            <span>${escapeHtml(t('video_only_safe'))}</span>
          </label>
          <label class="check">
            <input id="setting-check-updates" type="checkbox" ${state.settings?.check_updates ? 'checked' : ''} />
            <span>${escapeHtml(t('check_updates'))}</span>
          </label>
          <div class="field-row wrap">
            <button class="btn btn-ghost" id="copy-logs" type="button">${escapeHtml(t('copy_logs'))}</button>
            <button class="btn btn-ghost" id="reveal-logs" type="button">${escapeHtml(t('reveal_logs'))}</button>
            <button class="btn btn-ghost" id="copy-support" type="button">${escapeHtml(t('copy_support'))}</button>
            <button class="btn btn-ghost" id="check-updates-now" type="button">${escapeHtml(t('check_updates_now'))}</button>
            <button class="btn btn-ghost" id="reset-settings" type="button">${escapeHtml(t('reset_defaults'))}</button>
          </div>
        </div>
        <div class="modal-actions">
          <button class="btn btn-ghost" id="settings-cancel" type="button">${escapeHtml(t('cancel'))}</button>
          <button class="btn btn-primary" id="settings-save" type="button">${escapeHtml(t('save'))}</button>
        </div>
      </div>
    </div>

    <div class="modal-backdrop" id="audio-modal">
      <div class="modal">
        <h2>${escapeHtml(t('audio_pick_title'))}</h2>
        <p class="field-hint" id="audio-pick-title"></p>
        <p class="field-hint">${escapeHtml(t('audio_pick_hint'))}</p>
        <div class="field">
          <label>${escapeHtml(t('audio_track'))}</label>
          <select id="audio-track-select"></select>
        </div>
        <p class="field-hint task-warning" id="audio-ffmpeg-warn" hidden>${escapeHtml(t('audio_ffmpeg_missing'))}</p>
        <div class="modal-actions">
          <button class="btn btn-ghost" id="audio-cancel" type="button">${escapeHtml(t('cancel'))}</button>
          <button class="btn btn-primary" id="audio-confirm" type="button">${escapeHtml(t('continue_download'))}</button>
        </div>
      </div>
    </div>
  `

  state.shellReady = true
  bindShell()
  renderRestoreBanner()
  renderTaskList(true)
  const list = document.querySelector('#task-list')
  if (list) list.scrollTop = scrollTop
}

function qualityHint(preset: string): string {
  if (state.qualityHelp[preset]) return state.qualityHelp[preset]
  if (preset === 'best') return t('quality_help_best')
  if (preset === 'medium') return t('quality_help_medium')
  if (preset === 'audio') return t('quality_help_audio')
  return ''
}

function renderRestoreBanner(): void {
  const el = document.querySelector('#restore-banner')
  if (!el) return
  if (!state.restore) {
    el.className = 'restore-banner'
    el.innerHTML = ''
    return
  }
  el.className = 'restore-banner visible'
  el.innerHTML = `
    <div>
      <strong>${escapeHtml(t('restore_title'))}</strong>
      <div class="muted">${escapeHtml(t('restore_body'))} (${state.restore.incomplete}/${state.restore.total})</div>
    </div>
    <div class="field-row">
      <button class="btn btn-primary btn-sm" id="restore-confirm" type="button">${escapeHtml(t('restore_confirm'))}</button>
      <button class="btn btn-ghost btn-sm" id="restore-discard" type="button">${escapeHtml(t('restore_discard'))}</button>
    </div>
  `
  document.querySelector('#restore-confirm')?.addEventListener('click', async () => {
    const r = await api.confirmRestoreQueue()
    if (!r.ok) showNotice(r.error || 'Failed', 'error')
    else showNotice(r.message || t('restore_confirm'), 'success')
    state.restore = null
    renderRestoreBanner()
    await refreshTasks()
    renderTaskList(true)
  })
  document.querySelector('#restore-discard')?.addEventListener('click', async () => {
    await api.discardRestoreQueue()
    state.restore = null
    renderRestoreBanner()
  })
}

function renderTaskList(force = false): void {
  const list = document.querySelector('#task-list')
  if (!list) return
  const scrollTop = list.scrollTop
  const items = filteredTasks()
  const count = document.querySelector('#task-count')
  if (count) count.textContent = `${items.length} / ${state.tasks.length}`

  if (items.length === 0) {
    list.innerHTML = `<div class="empty">${escapeHtml(t('no_tasks'))}</div>`
    return
  }

  // Incremental: update existing nodes, append missing, remove stale.
  const seen = new Set<string>()
  const empty = list.querySelector('.empty')
  if (empty) empty.remove()

  // Ensure order matches filter list by rebuilding when force or large mismatch.
  const existing = Array.from(list.querySelectorAll<HTMLElement>('.task'))
  if (force || existing.length > items.length + 5) {
    list.innerHTML = items.map(renderTask).join('')
    bindTaskActions(list)
    list.scrollTop = scrollTop
    return
  }

  const byId = new Map(existing.map((el) => [el.dataset.id || '', el]))
  for (const task of items) {
    seen.add(task.id)
    const prev = byId.get(task.id)
    const html = renderTask(task)
    if (!prev) {
      list.insertAdjacentHTML('beforeend', html)
      const node = list.querySelector<HTMLElement>(`.task[data-id="${cssEscape(task.id)}"]`)
      if (node) bindTaskActions(node)
    } else if (prev.outerHTML !== html.trim()) {
      // Patch in place to keep listeners if structure same — replace node.
      prev.outerHTML = html
      const node = list.querySelector<HTMLElement>(`.task[data-id="${cssEscape(task.id)}"]`)
      if (node) bindTaskActions(node)
    }
  }
  for (const el of Array.from(list.querySelectorAll<HTMLElement>('.task'))) {
    if (!seen.has(el.dataset.id || '')) el.remove()
  }
  // Reorder lightly if needed
  let orderOk = true
  const nodes = Array.from(list.querySelectorAll<HTMLElement>('.task'))
  for (let i = 0; i < items.length && i < nodes.length; i++) {
    if (nodes[i].dataset.id !== items[i].id) {
      orderOk = false
      break
    }
  }
  if (!orderOk) {
    list.innerHTML = items.map(renderTask).join('')
    bindTaskActions(list)
  }
  list.scrollTop = scrollTop
}

function cssEscape(id: string): string {
  return id.replace(/"/g, '\\"')
}

function renderTask(task: DownloadTask): string {
  const canToggle =
    task.status === 'Pending' ||
    task.status === 'Error' ||
    task.status === 'Stopped' ||
    task.status === 'Paused' ||
    task.status === 'Downloading' ||
    task.status === 'Starting'

  const toggleLabel =
    task.status === 'Downloading' || task.status === 'Starting' ? t('pause') : t('continue')

  const hasFile = Boolean(task.output_path && !task.output_path.startsWith('http'))
  const hasSrt = Boolean(task.transcript_srt_path)
  const hasTxt = Boolean(task.transcript_txt_path)
  const transcriptBadge = transcriptBadgeLabel(task)
  const partial =
    task.status === 'Completed' &&
    Boolean(task.warning) &&
    (task.transcript_status === 'error' || task.transcript_status === 'unavailable')
  const canRetryTranscript =
    !task.status || ['Completed', 'Error', 'Stopped', 'Paused'].includes(task.status)
  const phase = phaseLabel(task.phase)

  return `
    <article class="task ${partial ? 'task-partial' : ''}" data-id="${escapeHtml(task.id)}">
      <div>
        <div class="task-title">${escapeHtml(task.title || task.url)}</div>
        <div class="task-meta">
          <span class="status ${statusClass(task.status)}">${escapeHtml(task.status)}</span>
          ${phase ? `<span class="badge badge-phase">${escapeHtml(phase)}</span>` : ''}
          ${partial ? `<span class="badge badge-warn">${escapeHtml(t('partial_success'))}</span>` : ''}
          ${transcriptBadge ? `<span class="badge ${partial ? 'badge-warn' : ''}">${escapeHtml(transcriptBadge)}</span>` : ''}
          <span>${task.percent || 0}%</span>
          <span>${escapeHtml(task.speed || '—')}</span>
          <span>ETA ${escapeHtml(formatETA(task.eta_sec))}</span>
        </div>
        <div class="progress"><span style="width:${Math.max(0, Math.min(100, task.percent || 0))}%"></span></div>
        ${task.warning ? `<div class="task-warning">${escapeHtml(task.warning)}</div>` : ''}
        ${task.last_error ? `<div class="task-error">${escapeHtml(task.last_error)}</div>` : ''}
        ${task.transcript_error && task.transcript_status !== 'completed' ? `<div class="task-error">${escapeHtml(task.transcript_error)}</div>` : ''}
      </div>
      <div class="actions">
        <button class="btn btn-icon" data-action="toggle" title="${escapeHtml(toggleLabel)}" ${canToggle ? '' : 'disabled'}>${
          task.status === 'Downloading' || task.status === 'Starting' ? '⏸' : '▶'
        }</button>
        <button class="btn btn-icon" data-action="reveal" title="${escapeHtml(t('reveal'))}" ${hasFile ? '' : 'disabled'}>📂</button>
        <button class="btn btn-icon" data-action="open" title="${escapeHtml(t('open'))}" ${hasFile ? '' : 'disabled'}>▶</button>
        <button class="btn btn-icon" data-action="reveal-srt" title="${escapeHtml(t('reveal_srt'))}" ${hasSrt ? '' : 'disabled'}>📁S</button>
        <button class="btn btn-icon" data-action="open-srt" title="${escapeHtml(t('open_srt'))}" ${hasSrt ? '' : 'disabled'}>SRT</button>
        <button class="btn btn-icon" data-action="reveal-txt" title="${escapeHtml(t('reveal_txt'))}" ${hasTxt ? '' : 'disabled'}>📁T</button>
        <button class="btn btn-icon" data-action="open-txt" title="${escapeHtml(t('open_txt'))}" ${hasTxt ? '' : 'disabled'}>TXT</button>
        <button class="btn btn-icon" data-action="copy-path" title="${escapeHtml(t('copy_path'))}" ${hasFile || hasSrt || hasTxt ? '' : 'disabled'}>📋</button>
        <button class="btn btn-icon" data-action="retry-transcript" title="${escapeHtml(t('retry_transcript'))}" ${canRetryTranscript ? '' : 'disabled'}>CC</button>
        <button class="btn btn-icon btn-danger" data-action="remove" title="${escapeHtml(t('remove'))}">✕</button>
      </div>
    </article>
  `
}

function transcriptBadgeLabel(task: DownloadTask): string {
  switch (task.transcript_status) {
    case 'pending':
      return t('transcript_pending')
    case 'completed':
      return t('transcript_done') + (task.transcript_lang ? ` (${task.transcript_lang})` : '')
    case 'unavailable':
      return t('transcript_missing')
    case 'error':
      return t('transcript_error')
    default:
      return ''
  }
}

function bindTaskActions(root: ParentNode): void {
  root.querySelectorAll<HTMLElement>('.task [data-action]').forEach((btn) => {
    if ((btn as HTMLElement & { _bound?: boolean })._bound) return
    ;(btn as HTMLElement & { _bound?: boolean })._bound = true
    btn.addEventListener('click', async () => {
      const action = btn.getAttribute('data-action')
      const taskEl = btn.closest<HTMLElement>('.task')
      const id = taskEl?.dataset.id
      if (!id || !action) return
      const task = state.tasks.find((item) => item.id === id)
      try {
        if (action === 'toggle') {
          const result = await api.togglePause(id)
          if (!result.ok) showNotice(result.error || 'Failed', 'error')
        } else if (action === 'remove') {
          const result = await api.removeTask(id)
          if (!result.ok) showNotice(result.error || 'Failed', 'error')
        } else if (action === 'open' && task?.output_path) {
          const result = await api.openFile(task.output_path)
          if (!result.ok) showNotice(result.error || 'Failed', 'error')
        } else if (action === 'reveal' && task?.output_path) {
          const result = await api.revealFile(task.output_path)
          if (!result.ok) showNotice(result.error || 'Failed', 'error')
        } else if (action === 'open-srt' && task?.transcript_srt_path) {
          const result = await api.openFile(task.transcript_srt_path)
          if (!result.ok) showNotice(result.error || 'Failed', 'error')
        } else if (action === 'reveal-srt' && task?.transcript_srt_path) {
          const result = await api.revealFile(task.transcript_srt_path)
          if (!result.ok) showNotice(result.error || 'Failed', 'error')
        } else if (action === 'open-txt' && task?.transcript_txt_path) {
          const result = await api.openFile(task.transcript_txt_path)
          if (!result.ok) showNotice(result.error || 'Failed', 'error')
        } else if (action === 'reveal-txt' && task?.transcript_txt_path) {
          const result = await api.revealFile(task.transcript_txt_path)
          if (!result.ok) showNotice(result.error || 'Failed', 'error')
        } else if (action === 'copy-path') {
          const path = task?.output_path || task?.transcript_txt_path || task?.transcript_srt_path || ''
          const result = await api.copyToClipboard(path)
          if (!result.ok) showNotice(result.error || 'Failed', 'error')
          else showNotice(t('copied'), 'success')
        } else if (action === 'retry-transcript') {
          const result = await api.retryTranscript(id)
          if (!result.ok) showNotice(result.error || 'Failed', 'error')
          else showNotice(result.message || t('retry_transcript'), 'info')
        }
        await refreshTasks()
        renderTaskList(true)
      } catch (err) {
        showNotice(String(err), 'error')
      }
    })
  })
}

async function startDownloadWithAudio(url: string, audioLanguage: string): Promise<void> {
  const input = document.querySelector<HTMLInputElement>('#url-input')
  const result = await api.startDownloadRequest({
    url,
    mode: state.mode,
    transcript_language: state.transcriptLang || 'auto',
    audio_language: audioLanguage || undefined,
  })
  if (!result.ok) {
    showNotice(result.error || 'Failed', 'error')
    return
  }
  showNotice(result.message || 'OK', 'success')
  if (input) input.value = ''
  await refreshTasks()
  renderTaskList(true)
  hideNoticeSoon()
}

function openAudioPicker(url: string, title: string, tracks: AudioTrack[], ffmpegMissing: boolean): void {
  state.pendingURL = url
  state.audioTracks = tracks
  state.audioPickTitle = title
  state.audioFFmpegMissing = ffmpegMissing
  const select = document.querySelector<HTMLSelectElement>('#audio-track-select')
  const titleEl = document.querySelector('#audio-pick-title')
  const warn = document.querySelector<HTMLElement>('#audio-ffmpeg-warn')
  if (titleEl) titleEl.textContent = title || url
  if (select) {
    select.innerHTML = tracks
      .map((tr) => {
        const value = tr.track_id || tr.language_code || ''
        const label =
          (tr.display_name || tr.language_code || t('audio_default')) +
          (tr.is_default ? ` (${t('audio_default')})` : '')
        return `<option value="${escapeHtml(value)}">${escapeHtml(label)}</option>`
      })
      .join('')
    const def = tracks.find((tr) => tr.is_default) || tracks[0]
    if (def) select.value = def.track_id || def.language_code || ''
  }
  if (warn) warn.hidden = !ffmpegMissing
  document.querySelector('#audio-modal')?.classList.add('open')
}

function bindShell(): void {
  document.querySelector('#paste-url-btn')?.addEventListener('click', async () => {
    const input = document.querySelector<HTMLInputElement>('#url-input')
    if (!input) return
    try {
      let text = ''
      try {
        const result = await api.getClipboardText()
        if (result.ok && result.message) text = result.message
        else if (result.error) throw new Error(result.error)
      } catch {
        // Fallback for browser/dev without Wails clipboard bridge.
        if (navigator.clipboard?.readText) {
          text = await navigator.clipboard.readText()
        }
      }
      text = (text || '').replace(/[\r\n\t]+/g, ' ').trim()
      if (!text) {
        showNotice(t('paste_empty'), 'error')
        return
      }
      input.value = text
      input.focus()
      input.select()
      showNotice(t('pasted'), 'success')
      hideNoticeSoon()
    } catch (err) {
      showNotice(String(err), 'error')
    }
  })

  document.querySelector('#download-form')?.addEventListener('submit', async (event) => {
    event.preventDefault()
    const input = document.querySelector<HTMLInputElement>('#url-input')
    const url = input?.value.trim() || ''
    if (!url) {
      showNotice(t('empty_url'), 'error')
      return
    }

    state.busy = true
    const btn = document.querySelector<HTMLButtonElement>('#download-btn')
    if (btn) btn.disabled = true
    try {
      // Playlists skip audio probe; single videos may offer multi-language audio.
      const isPlaylist = url.includes('list=')
      const needsMedia = state.mode !== 'transcript_only'
      if (!isPlaylist && needsMedia) {
        showNotice(t('probing'), 'info', true)
        const probe = await api.probeMedia(url)
        if (!probe.ok) {
          showNotice(probe.error || 'Probe failed', 'error')
          return
        }
        const tracks = probe.audio_tracks || []
        if (tracks.length > 1) {
          hideNoticeSoon()
          openAudioPicker(url, probe.title || '', tracks, Boolean(probe.needs_ffmpeg && !probe.ffmpeg_found))
          return
        }
      }
      await startDownloadWithAudio(url, '')
    } catch (err) {
      showNotice(String(err), 'error')
    } finally {
      state.busy = false
      if (btn) btn.disabled = false
    }
  })

  document.querySelector('#audio-cancel')?.addEventListener('click', () => {
    document.querySelector('#audio-modal')?.classList.remove('open')
    state.pendingURL = ''
  })
  document.querySelector('#audio-confirm')?.addEventListener('click', async () => {
    const url = state.pendingURL
    const select = document.querySelector<HTMLSelectElement>('#audio-track-select')
    const audioLang = select?.value || ''
    document.querySelector('#audio-modal')?.classList.remove('open')
    if (!url) return
    state.busy = true
    try {
      await startDownloadWithAudio(url, audioLang)
    } catch (err) {
      showNotice(String(err), 'error')
    } finally {
      state.busy = false
      state.pendingURL = ''
    }
  })

  document.querySelectorAll<HTMLButtonElement>('[data-filter]').forEach((btn) => {
    btn.addEventListener('click', () => {
      state.filter = btn.dataset.filter as StatusFilter
      document.querySelectorAll('[data-filter]').forEach((b) => b.classList.remove('active'))
      btn.classList.add('active')
      renderTaskList(true)
    })
  })

  document.querySelectorAll<HTMLButtonElement>('[data-mode]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      state.mode = (btn.dataset.mode || 'video') as DownloadMode
      if (state.settings) state.settings.download_mode = state.mode
      document.querySelectorAll('[data-mode]').forEach((b) => b.classList.remove('active'))
      btn.classList.add('active')
      // Persist mode default so settings UI stays in sync (E3).
      if (state.settings) {
        try {
          await api.saveSettings({ ...state.settings, download_mode: state.mode })
        } catch {
          /* ignore */
        }
      }
    })
  })

  document.querySelector<HTMLSelectElement>('#transcript-lang')?.addEventListener('change', async (ev) => {
    const v = (ev.target as HTMLSelectElement).value || 'auto'
    state.transcriptLang = v
    if (state.settings) {
      state.settings.transcript_language = v
      try {
        await api.saveSettings({ ...state.settings, transcript_language: v })
      } catch {
        /* ignore */
      }
    }
  })

  document.querySelector('#pause-all')?.addEventListener('click', () =>
    runAction(() => api.pauseAll(), t('pause_all')),
  )
  document.querySelector('#clear-completed')?.addEventListener('click', () =>
    runAction(() => api.clearCompleted(), t('clear_completed'), 'success'),
  )
  document.querySelector('#retry-failed')?.addEventListener('click', () =>
    runAction(() => api.retryFailed(), t('retry_failed')),
  )

  document.querySelector('#settings-btn')?.addEventListener('click', () => {
    document.querySelector('#settings-modal')?.classList.add('open')
  })
  document.querySelector('#settings-cancel')?.addEventListener('click', () => {
    document.querySelector('#settings-modal')?.classList.remove('open')
  })
  document.querySelector('#browse-dir')?.addEventListener('click', async () => {
    try {
      const dir = await api.selectDownloadDirectory()
      const input = document.querySelector<HTMLInputElement>('#setting-dir')
      if (input && dir) input.value = dir
    } catch (err) {
      showNotice(String(err), 'error')
    }
  })
  document.querySelector('#open-dir')?.addEventListener('click', async () => {
    const r = await api.openDownloadFolder()
    if (!r.ok) showNotice(r.error || 'Failed', 'error')
  })
  document.querySelector('#setting-quality')?.addEventListener('change', (ev) => {
    const v = (ev.target as HTMLSelectElement).value
    const hint = document.querySelector('#quality-hint')
    if (hint) hint.textContent = qualityHint(v)
  })
  document.querySelector('#copy-logs')?.addEventListener('click', async () => {
    const result = await api.copyLogs()
    if (!result.ok) showNotice(result.error || 'Failed', 'error')
    else showNotice(t('logs_copied'), 'success')
  })
  document.querySelector('#reveal-logs')?.addEventListener('click', async () => {
    const result = await api.revealLogFile()
    if (!result.ok) showNotice(result.error || 'Failed', 'error')
  })
  document.querySelector('#copy-support')?.addEventListener('click', async () => {
    const result = await api.copySupportBundle()
    if (!result.ok) showNotice(result.error || 'Failed', 'error')
    else showNotice(t('support_copied'), 'success')
  })
  document.querySelector('#check-updates-now')?.addEventListener('click', async () => {
    try {
      const res = await api.checkForUpdates()
      if (res.error) {
        showNotice(res.error, 'warn')
        return
      }
      if (res.update_available) {
        showNotice(
          `${t('update_available')}: v${res.latest || '?'} (v${res.current || state.version})`,
          'info',
        )
      } else {
        showNotice(t('update_latest'), 'success')
      }
    } catch (err) {
      showNotice(String(err), 'error')
    }
  })
  document.querySelector('#reset-settings')?.addEventListener('click', async () => {
    const result = await api.resetSettings()
    if (!result.ok) {
      showNotice(result.error || 'Failed', 'error')
      return
    }
    state.settings = await api.getSettings()
    state.mode = state.settings?.download_mode || 'video'
    state.transcriptLang = state.settings?.transcript_language || 'auto'
    setLang(state.settings?.app_language || 'lt')
    document.querySelector('#settings-modal')?.classList.remove('open')
    showNotice(result.message || t('settings_saved'), 'success')
    renderShell()
  })
  document.querySelector('#settings-save')?.addEventListener('click', async () => {
    if (!state.settings) return
    const parallel = Number(valueOf('#setting-parallel') || '2')
    if (!Number.isFinite(parallel) || parallel < 1 || parallel > 10) {
      showNotice('Max parallel must be 1–10', 'error')
      return
    }
    let quality = valueOf('#setting-quality') || 'medium'
    if (quality === 'audio') quality = 'medium'
    const next: SettingsData = {
      download_directory: valueOf('#setting-dir'),
      max_parallel_downloads: parallel,
      quality_preset: quality,
      filename_template: valueOf('#setting-template') || '%(title)s.%(ext)s',
      app_language: valueOf('#setting-lang') || 'lt',
      auto_reveal_on_complete: Boolean(
        document.querySelector<HTMLInputElement>('#setting-reveal')?.checked,
      ),
      download_mode: (valueOf('#setting-mode') || 'video') as DownloadMode,
      transcript_language: valueOf('#setting-transcript-lang') || 'auto',
      collision_policy: valueOf('#setting-collision') || 'rename',
      log_level: valueOf('#setting-log-level') || 'info',
      log_to_file: Boolean(document.querySelector<HTMLInputElement>('#setting-log-file')?.checked),
      disable_captions: Boolean(
        document.querySelector<HTMLInputElement>('#setting-disable-captions')?.checked,
      ),
      video_only_safe_mode: Boolean(
        document.querySelector<HTMLInputElement>('#setting-video-only')?.checked,
      ),
      check_updates: Boolean(document.querySelector<HTMLInputElement>('#setting-check-updates')?.checked),
    }
    if (!next.download_directory.trim()) {
      showNotice('Download directory is required', 'error')
      return
    }
    try {
      const result = await api.saveSettings(next)
      if (!result.ok) {
        showNotice(result.error || 'Failed to save', 'error')
        return
      }
      state.settings = next
      state.mode = next.download_mode || 'video'
      state.transcriptLang = next.transcript_language || 'auto'
      setLang(next.app_language)
      document.querySelector('#settings-modal')?.classList.remove('open')
      showNotice(t('settings_saved'), 'success')
      hideNoticeSoon()
      renderShell()
    } catch (err) {
      showNotice(String(err), 'error')
    }
  })
}

function valueOf(selector: string): string {
  return document.querySelector<HTMLInputElement | HTMLSelectElement>(selector)?.value || ''
}

async function refreshTasks(): Promise<void> {
  state.tasks = (await api.getTasks()) || []
}

async function refreshAndRenderTasks(force = true): Promise<void> {
  await refreshTasks()
  renderTaskList(force)
}

function upsertTask(task: DownloadTask): void {
  const idx = state.tasks.findIndex((item) => item.id === task.id)
  if (idx >= 0) state.tasks[idx] = task
  else state.tasks.unshift(task)
}

async function bootstrap(): Promise<void> {
  renderShell()
  try {
    state.version = await api.getVersion()
    state.settings = await api.getSettings()
    state.mode = state.settings?.download_mode || 'video'
    state.transcriptLang = state.settings?.transcript_language || 'auto'
    setLang(state.settings?.app_language || 'lt')
    try {
      state.qualityHelp = (await api.getQualityHelp()) || {}
    } catch {
      state.qualityHelp = {}
    }
    await refreshTasks()
    renderShell()
    try {
      const info = await api.getRestoreInfo()
      if (info && (info as { pending?: boolean }).pending) {
        state.restore = { total: info.total || 0, incomplete: info.incomplete || 0 }
        renderRestoreBanner()
      }
    } catch {
      /* optional API */
    }
  } catch (err) {
    showNotice(String(err), 'error')
  }

  eventsOn('task:update', (payload) => {
    upsertTask(payload as DownloadTask)
    // Incremental row update — avoid full shell rebuild on progress ticks (E2).
    renderTaskList(false)
  })
  eventsOn('tasks:changed', async () => {
    const list = document.querySelector('#task-list')
    const scrollTop = list?.scrollTop ?? 0
    await refreshTasks()
    renderTaskList(true)
    if (list) list.scrollTop = scrollTop
  })
  eventsOn('notification', (payload) => {
    const n = payload as NotificationPayload
    showNotice(n.message, n.level || 'info', Boolean(n.spinning))
    if (!n.spinning) hideNoticeSoon()
  })
  eventsOn('download:complete', (payload) => {
    const task = payload as DownloadTask
    showNotice(`${t('completed')}: ${task.title || task.url}`, 'success')
    hideNoticeSoon()
  })
  eventsOn('queue:restore', (payload) => {
    state.restore = payload as QueueRestorePayload
    renderRestoreBanner()
  })
}

bootstrap()
