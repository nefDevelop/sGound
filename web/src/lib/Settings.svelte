<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import { audioQuality, showToast } from './stores.js';
  import { generatePalette, applyTheme } from './theme.js';
  import { copyToClipboard } from './utils.js';
  import Modal from './Modal.svelte';
  import PatternBuilder from './PatternBuilder.svelte';
  
  let config = $state({ musicPath: '', autoRename: false, renamePattern: '', themeColor: '#ff3366', replayGainEnabled: false, language: 'es' });
  let loading = $state(false);
  let saveSuccess = $state(false);

  let { onscan } = $props();

  // Derivamos la paleta automáticamente de la semilla
  let palette = $derived(generatePalette(config.themeColor));

  onMount(async () => {
    const res = await fetch('/api/config');
    const data = await res.json();
    config.musicPath = data.musicPath;
    config.autoRename = data.autoRename;
    config.renamePattern = data.renamePattern || '{artist}/{album}/{track} - {title}';
    config.themeColor = data.themeColor || '#ff3366';
    config.language = data.language || 'es';
    applyTheme(palette);
    fetchAPIKeys();
  });

  let apiKeys = $state([]);
  let newKeyLabel = $state('');
  let newKeyPerms = $state('r');
  let myPerms = $derived(localStorage.getItem('permissions') || 'r');
  
  let newlyCreatedKey = $state(null);
  let showKeyModal = $state(false);

  async function fetchAPIKeys() {
    const res = await fetch('/api/auth/apikeys');
    if (res.ok) apiKeys = await res.json();
  }

  async function createAPIKey() {
    if (!newKeyLabel) return;
    const res = await fetch('/api/auth/apikeys', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ label: newKeyLabel, permissions: newKeyPerms })
    });
    if (res.ok) {
      const data = await res.json();
      newlyCreatedKey = data.key;
      showKeyModal = true;
      newKeyLabel = '';
      fetchAPIKeys();
      showToast("API Key generated successfully", "success");
    } else {
      showToast("Failed to generate API Key", "error");
    }
  }

  async function deleteAPIKey(key) {
    await fetch(`/api/auth/apikeys?key=${key}`, { method: 'DELETE' });
    fetchAPIKeys();
  }

  async function toggleAPIKeyStatus(key, currentStatus) {
    const res = await fetch('/api/auth/apikeys', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ key, active: !currentStatus })
    });
    if (res.ok) fetchAPIKeys();
  }

  function getDeviceInfo(ua) {
    if (!ua) return { type: 'Unknown', icon: 'helpCircle' };
    const lowerUA = ua.toLowerCase();
    if (lowerUA.includes('android') || lowerUA.includes('iphone') || lowerUA.includes('ipad')) {
      return { type: 'Mobile', icon: 'smartphone' };
    }
    if (lowerUA.includes('windows') || lowerUA.includes('macintosh') || lowerUA.includes('linux')) {
      return { type: 'PC', icon: 'monitor' };
    }
    return { type: 'Device', icon: 'sparkles' };
  }

  function getRelativeTime(dateStr) {
    if (!dateStr || dateStr === '' || dateStr.startsWith('0001')) return 'Never used';
    const date = new Date(dateStr);
    const now = new Date();
    const diff = Math.floor((now.getTime() - date.getTime()) / 1000);
    
    if (diff < 60) return 'Just now';
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
    return date.toLocaleDateString();
  }

  // Efecto para aplicar el tema cuando cambia la paleta (Svelte 5)
  $effect(() => {
    applyTheme(palette);
  });

  let organizeStatus = $state({ isRunning: false, total: 0, processed: 0, current: '', errors: [] });
  let showPreviewModal = $state(false);
  let previews = $state([]);
  let loadingPreview = $state(false);

  async function pollOrganizeStatus() {
    const res = await fetch('/api/library/organize/status');
    if (res.ok) {
      organizeStatus = await res.json();
      if (organizeStatus.isRunning) {
        setTimeout(pollOrganizeStatus, 1000);
      }
    }
  }

  $effect(() => {
    pollOrganizeStatus();
  });

  async function fetchPreview() {
    loadingPreview = true;
    showPreviewModal = true;
    try {
      const res = await fetch('/api/library/organize/preview');
      if (res.ok) previews = await res.json();
    } catch (e) {
      showToast("Failed to fetch preview", "error");
    } finally {
      loadingPreview = false;
    }
  }

  async function startOrganize() {
    showPreviewModal = false;
    const res = await fetch('/api/library/organize', { method: 'POST' });
    if (res.ok) {
      pollOrganizeStatus();
    } else {
      showToast("Failed to start organization", "error");
    }
  }

  async function saveConfig() {
    loading = true;
    try {
      const res = await fetch('/api/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config)
      });
      if (res.ok) {
        showToast("Configuration saved", "success");
        saveSuccess = true;
        setTimeout(() => saveSuccess = false, 2000);
      }
    } catch (e) { 
      showToast("Failed to save config", "error"); 
    } finally { 
      loading = false; 
    }
  }
</script>

<div class="settings-view">
  <h1>Settings</h1>
  
  <div class="settings-grid">
    <div class="settings-column">
      <div class="settings-card">
        <h2>Appearance & Theme</h2>
        <section class="setting-item">
          <div class="info">
            <div class="title-with-help">
              <h3>Theme Seed Color</h3>
              <div class="tooltip-wrap">
                <Icon name="info" size={14} color="var(--text-muted)" />
                <span class="tooltip">Define el color base para la paleta.</span>
              </div>
            </div>
            <p>Generate a unique Material Design 3 palette.</p>
          </div>
          <div class="color-section">
              <div class="color-picker-custom">
                  <label class="color-swatch" style="background-color: {config.themeColor}">
                    <input type="color" bind:value={config.themeColor} />
                  </label>
                  <span class="hex-val">{config.themeColor}</span>
              </div>
          </div>
        </section>

        <section class="setting-item">
          <div class="info">
            <h3>Palette Preview</h3>
            <div class="palette-preview">
              <div class="swatch-group">
                <div class="mini-swatch" style="background-color: {palette.primary}"></div>
                <span>Pri</span>
              </div>
              <div class="swatch-group">
                <div class="mini-swatch" style="background-color: {palette.secondary}"></div>
                <span>Sec</span>
              </div>
              <div class="swatch-group">
                <div class="mini-swatch" style="background-color: {palette.tertiary}"></div>
                <span>Ter</span>
              </div>
            </div>
          </div>
        </section>
      </div>

      <div class="settings-card">
        <h2>Library & Playback</h2>
        <section class="setting-item">
          <div class="info">
            <h3>Music Path</h3>
            <p>Root directory for your audio files.</p>
          </div>
          <input type="text" bind:value={config.musicPath} placeholder="/path/to/music" />
        </section>

        <section class="setting-item">
          <div class="info">
            <div class="title-with-help">
              <h3>Auto Cataloging</h3>
              <div class="tooltip-wrap">
                <Icon name="helpCircle" size={14} />
                <div class="tooltip">Automatically renames and moves files based on tags.</div>
              </div>
            </div>
            <p>Automatically move files when tags are updated.</p>
          </div>
          <div style="display: flex; align-items: center; gap: 1rem;">
             <input type="checkbox" bind:checked={config.autoRename} style="width: 20px; height: 20px;" />
             <span style="font-weight: 700;">Enable Automatic Organization</span>
          </div>
        </section>

        {#if config.autoRename}
        <section class="setting-item">
          <div class="info">
            <h3>Rename Pattern</h3>
            <p>Define how files will be structured using tags and separators.</p>
          </div>
          <PatternBuilder bind:pattern={config.renamePattern} />
          
          {#if organizeStatus.isRunning || (organizeStatus.processed > 0 && organizeStatus.processed < organizeStatus.total)}
            <div class="organize-progress-wrap">
              <div class="progress-bar-bg">
                 <div class="progress-bar-fill" style="width: {(organizeStatus.processed / (organizeStatus.total || 1)) * 100}%"></div>
              </div>
              <div class="progress-stats">
                 <span class="p-count">{organizeStatus.processed} / {organizeStatus.total} tracks</span>
                 <span class="p-current">{organizeStatus.isRunning ? 'Moving: ' + organizeStatus.current : 'Done'}</span>
              </div>
              {#if organizeStatus.errors && organizeStatus.errors.length > 0}
                <div class="organize-errors">
                  <h4>Errors ({organizeStatus.errors.length})</h4>
                  <ul>
                    {#each organizeStatus.errors.slice(-5) as err}
                      <li>{err}</li>
                    {/each}
                  </ul>
                </div>
              {/if}
            </div>
          {:else}
            <button class="scan-btn" onclick={fetchPreview}>
               <Icon name="folder" size={18} />
               Organize Library Now
            </button>
          {/if}
        </section>
        {/if}

        <section class="setting-item">
          <div class="info">
            <h3>Audio Quality</h3>
            <p>Choose the bitrate for streaming.</p>
          </div>
          <select bind:value={$audioQuality} class="select-input">
            <option value="auto">Auto-detect</option>
            <option value="original">Original</option>
            <option value="320k">320 kbps</option>
            <option value="192k">192 kbps</option>
            <option value="128k">128 kbps</option>
          </select>
        </section>
      </div>

      <div class="settings-card">
        <h2>Audio & Playback</h2>
        <section class="setting-item">
          <div class="info">
            <div class="title-with-help">
              <h3>Normalización de Volumen</h3>
              <div class="tooltip-wrap">
                <Icon name="info" size={14} color="var(--text-muted)" />
                <span class="tooltip">Aplica ReplayGain para que todas las canciones suenen al mismo nivel.</span>
              </div>
            </div>
            <p>Utiliza FFmpeg para ajustar el volumen según los metadatos del archivo.</p>
          </div>
          <div class="control">
            <label class="switch">
              <input type="checkbox" bind:checked={config.replayGainEnabled} />
              <span class="slider"></span>
            </label>
          </div>
        </section>
      </div>
    </div>

    <div class="settings-column">
      <div class="settings-card">
        <h2>Metadata & Language</h2>
        
        <section class="setting-item">
          <div class="info">
            <h3>Idioma de Metadatos</h3>
            <p>Idioma preferido para biografías y descripciones (Wikipedia / Last.fm).</p>
          </div>
          <select bind:value={config.language} class="select-input">
            <option value="es">Español</option>
            <option value="en">English</option>
          </select>
        </section>
      </div>

      <div class="settings-card">
        <h2>Maintenance</h2>
        <section class="setting-item">
          <div class="info">
            <h3>Scan Library</h3>
            <p>Index new files or update metadata.</p>
          </div>
          <button class="scan-btn" onclick={onscan}>
             <Icon name="refresh" size={18} />
             Scan Now
          </button>
        </section>
      </div>

      <div class="settings-card">
        <h2>API Keys</h2>
        <p class="section-desc">Tokens for external apps and mobile clients.</p>
        
        <div class="api-keys-list">
          {#each apiKeys as key}
            {@const device = getDeviceInfo(key.user_agent)}
            <div class="key-item {key.is_active ? '' : 'blocked'}">
              <div class="key-main">
                <div class="key-info">
                  <div class="label-row">
                    <span class="key-label">{key.label}</span>
                    <span class="badge {key.permissions}">{key.permissions}</span>
                    {#if !key.is_active}
                      <span class="badge blocked">BLOCKED</span>
                    {/if}
                  </div>
                  <div class="monitoring-info">
                    <div class="mon-item">
                      <Icon name={device.icon} size={12} color="var(--text-muted)" />
                      <span>{device.type}</span>
                    </div>
                    {#if key.last_ip}
                      <div class="mon-item">
                        <Icon name="hash" size={12} color="var(--text-muted)" />
                        <span>{key.last_ip}</span>
                      </div>
                    {/if}
                    <div class="mon-item">
                      <Icon name="clock" size={12} color="var(--text-muted)" />
                      <span>{getRelativeTime(key.last_used_at)}</span>
                    </div>
                  </div>
                </div>
                <div class="key-actions">
                  <button class="icon-btn" onclick={() => copyToClipboard(key.key).then(ok => ok && showToast("Key copied", "info"))} title="Copy"><Icon name="copy" size={16} /></button>
                  <button class="icon-btn {key.is_active ? '' : 'active'}" onclick={() => toggleAPIKeyStatus(key.key, key.is_active)} title={key.is_active ? "Block" : "Unblock"}>
                    <Icon name={key.is_active ? "lock" : "eye"} size={16} />
                  </button>
                  <button class="icon-btn delete" onclick={() => deleteAPIKey(key.key)} title="Delete"><Icon name="trash" size={16} /></button>
                </div>
              </div>
            </div>
          {/each}
        </div>

        <div class="add-key-form">
          <input type="text" placeholder="Device name" bind:value={newKeyLabel} />
          <select bind:value={newKeyPerms} class="perm-select">
            <option value="r">Read Only</option>
            <option value="rw" disabled={myPerms === 'r'}>Full Access (RW)</option>
          </select>
          <button class="add-btn" onclick={createAPIKey}>Generate</button>
        </div>
      </div>
    </div>
  </div>

  <div class="global-actions">
    <button class="save-btn" onclick={saveConfig} disabled={loading}>
      {loading ? 'Saving...' : saveSuccess ? 'Saved' : 'Save Configuration'}
      {#if saveSuccess}<Icon name="check" size={16} color="white" />{/if}
    </button>
  </div>

  <Modal bind:isOpen={showKeyModal} title="API Key Generated" width="450px">
    <div class="new-key-display">
      <p>Please copy your new API key now. You won't be able to see it again easily.</p>
      <div class="key-box">
        <code>{newlyCreatedKey}</code>
        <button class="icon-btn" onclick={() => { copyToClipboard(newlyCreatedKey); showToast("Copied to clipboard", "info"); }}>
          <Icon name="copy" size={20} />
        </button>
      </div>
      <button class="btn-primary-rounded" onclick={() => showKeyModal = false}>Got it</button>
    </div>
  </Modal>

  <Modal bind:isOpen={showPreviewModal} title="Organize Library Preview" width="800px">
    <div class="preview-content">
      <p>Here's how some of your files will be renamed. This is a <b>preview</b> of the first 50 tracks.</p>
      
      {#if loadingPreview}
        <div class="loading-preview">
          <Icon name="refresh" size={32} class="spin" />
          <span>Calculating paths...</span>
        </div>
      {:else}
        <div class="preview-list">
          {#each previews as item}
            <div class="preview-item">
              <div class="p-title">{item.title}</div>
              <div class="p-paths">
                <div class="p-old"><Icon name="file" size={12} /> {item.oldPath.split('/').pop()}</div>
                <div class="p-arrow"><Icon name="arrowRight" size={14} /></div>
                <div class="p-new"><Icon name="folder" size={12} /> {item.newPath.replace(config.musicPath, '')}</div>
              </div>
            </div>
          {/each}
        </div>

        <div class="preview-actions">
          <button class="btn-secondary" onclick={() => showPreviewModal = false}>Cancel</button>
          <button class="btn-primary-rounded" onclick={startOrganize}>Confirm & Organize All</button>
        </div>
      {/if}
    </div>
  </Modal>
</div>

<style>
  .preview-content { display: flex; flex-direction: column; gap: 1.5rem; max-height: 70vh; }
  .preview-list { overflow-y: auto; display: flex; flex-direction: column; gap: 0.8rem; padding-right: 0.5rem; }
  .preview-item { background: var(--surface-container-low); padding: 1rem; border-radius: 12px; border: 1px solid var(--border-color); }
  .p-title { font-weight: 800; margin-bottom: 0.5rem; color: var(--primary); }
  .p-paths { display: flex; align-items: center; gap: 1rem; font-size: 0.8rem; color: var(--text-muted); }
  .p-old, .p-new { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }
  .p-new { color: var(--text-main); font-weight: 600; }
  .p-arrow { flex-shrink: 0; color: var(--primary); }

  .loading-preview { display: flex; flex-direction: column; align-items: center; gap: 1rem; padding: 3rem; }
  .preview-actions { display: flex; justify-content: flex-end; gap: 1rem; margin-top: 1rem; }
  .btn-secondary { background: none; border: 1px solid var(--border-color); color: var(--text-main); padding: 0.8rem 1.5rem; border-radius: 100px; font-weight: 700; cursor: pointer; }

  .organize-errors {
    margin-top: 0.5rem;
    background: rgba(255, 0, 0, 0.05);
    border: 1px solid rgba(255, 0, 0, 0.2);
    border-radius: 12px;
    padding: 0.8rem;
  }
  .organize-errors h4 { margin: 0 0 0.5rem 0; font-size: 0.8rem; color: #ff5555; text-transform: uppercase; }
  .organize-errors ul { margin: 0; padding-left: 1.2rem; font-size: 0.75rem; color: var(--text-muted); }
  .organize-errors li { margin-bottom: 0.2rem; }

  .spin { animation: spin 2s linear infinite; }
  @keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
  .settings-view { width: 100%; padding: 0 0 4rem 0; animation: fadeIn 0.4s ease-out; }
  h1 { font-size: 2.5rem; font-weight: 900; letter-spacing: -1.5px; margin: 0 0 2.5rem 0; color: var(--text-main); }
  
  .settings-grid {
    display: grid;
    grid-template-columns: 1fr;
    gap: 2rem;
    align-items: start;
  }

  .settings-column {
    display: flex;
    flex-direction: column;
    gap: 2rem;
  }

  @media (min-width: 1100px) {
    .settings-grid { grid-template-columns: 1fr 1fr; }
  }

  .settings-card {
    background: var(--surface-container); 
    border-radius: 32px; 
    padding: 2.5rem;
    border: 1px solid var(--border-color); 
    display: flex; 
    flex-direction: column; 
    gap: 2.5rem;
  }

  .setting-item { display: flex; flex-direction: column; gap: 1.5rem; }
  
  .info h3 { margin: 0 0 0.5rem 0; font-size: 1.2rem; font-weight: 800; color: var(--text-main); }
  .info p { margin: 0; color: var(--text-muted); font-size: 0.95rem; line-height: 1.6; }

  .title-with-help { display: flex; align-items: center; gap: 0.6rem; }
  .tooltip-wrap { position: relative; cursor: help; }
  .tooltip {
    visibility: hidden; position: absolute; bottom: 130%; left: 50%; transform: translateX(-50%);
    background: var(--surface-container-highest); color: var(--text-main); padding: 0.6rem 1rem;
    border-radius: 12px; font-size: 0.8rem; white-space: nowrap; z-index: 10; opacity: 0;
    transition: all 0.2s; border: 1px solid var(--border-color);
  }
  .tooltip-wrap:hover .tooltip { visibility: visible; opacity: 1; transform: translateX(-50%) translateY(-5px); }

  .color-picker-custom {
      display: flex; align-items: center; gap: 1.2rem;
      background: var(--surface-container-highest); padding: 0.6rem 1.2rem;
      border-radius: 100px; border: 1px solid var(--border-color); width: fit-content;
  }

  .color-swatch {
      width: 32px; height: 32px; border-radius: 50%; cursor: pointer;
      display: block; border: 2px solid rgba(255,255,255,0.2);
      position: relative; overflow: hidden;
  }
  .color-swatch input { position: absolute; opacity: 0; cursor: pointer; }

  .palette-preview { display: flex; gap: 1rem; margin-top: 0.5rem; }
  .swatch-group { display: flex; align-items: center; gap: 0.5rem; }
  .swatch-group span { font-size: 0.7rem; font-weight: 700; color: var(--text-muted); }
  .mini-swatch { width: 16px; height: 16px; border-radius: 4px; border: 1px solid rgba(255,255,255,0.1); }

  input[type="text"], .select-input {
    background: var(--surface-container-high); border: 1px solid var(--border-color);
    color: var(--text-main); padding: 1rem 1.5rem; border-radius: 16px; width: 100%; outline: none;
    font-weight: 600; transition: all 0.2s;
  }
  input[type="text"]:focus { border-color: var(--primary); }

  .scan-btn {
    background: var(--surface-container-highest); color: var(--primary);
    border: 1px solid var(--primary-30); padding: 1rem 2rem; border-radius: 100px;
    font-weight: 800; cursor: pointer; display: flex; align-items: center; gap: 0.8rem; transition: all 0.2s;
  }
  .scan-btn:hover { background: var(--primary-10); border-color: var(--primary); }

  .organize-progress-wrap {
    background: var(--surface-container-low);
    padding: 1.5rem;
    border-radius: 20px;
    border: 1px solid var(--border-color);
    display: flex;
    flex-direction: column;
    gap: 1rem;
    animation: fadeIn 0.3s ease-out;
  }

  .progress-bar-bg {
    height: 8px;
    background: rgba(255,255,255,0.05);
    border-radius: 4px;
    overflow: hidden;
  }

  .progress-bar-fill {
    height: 100%;
    background: var(--primary);
    box-shadow: 0 0 15px var(--primary-40);
    transition: width 0.3s ease-out;
  }

  .progress-stats {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 0.85rem;
  }

  .p-count { font-weight: 800; color: var(--primary); }
  .p-current { color: var(--text-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 60%; }

  .api-keys-list { display: flex; flex-direction: column; gap: 1rem; }
  .key-item { 
    padding: 1.2rem; background: var(--surface-container-low); border-radius: 20px; border: 1px solid var(--border-color);
    transition: all 0.2s;
  }
  .key-item.blocked { opacity: 0.6; background: var(--surface-container-lowest); border-color: rgba(255,255,255,0.05); }
  .key-main { display: flex; justify-content: space-between; align-items: flex-start; gap: 1rem; }
  .key-info { display: flex; flex-direction: column; gap: 0.6rem; flex: 1; min-width: 0; }
  .label-row { display: flex; align-items: center; gap: 0.8rem; flex-wrap: wrap; }
  .key-label { font-weight: 800; color: var(--text-main); font-size: 1.05rem; }
  .monitoring-info { display: flex; gap: 1.2rem; flex-wrap: wrap; }
  .mon-item { display: flex; align-items: center; gap: 0.4rem; color: var(--text-muted); font-size: 0.75rem; font-weight: 600; }
  .mon-item span { white-space: nowrap; }
  
  .badge { padding: 0.2rem 0.5rem; border-radius: 6px; font-size: 0.7rem; font-weight: 900; width: fit-content; text-transform: uppercase; }
  .badge.rw { background: var(--primary-10); color: var(--primary); }
  .badge.blocked { background: rgba(255,255,255,0.1); color: var(--text-muted); border: 1px solid rgba(255,255,255,0.1); }
  
  .key-actions { display: flex; gap: 0.5rem; }
  .icon-btn.active { color: var(--primary); }
  
  .add-key-form { margin-top: 1.5rem; display: flex; gap: 0.8rem; }
  .perm-select {
    background: var(--surface-container-low);
    border: 1px solid var(--border-color);
    color: var(--text-main);
    padding: 0 1rem;
    border-radius: 12px;
    font-weight: 600;
    outline: none;
    cursor: pointer;
  }
  .add-btn { background: var(--secondary); color: white; border: none; padding: 0.8rem 1.5rem; border-radius: 12px; font-weight: 700; cursor: pointer; }

  .global-actions { margin-top: 3rem; display: flex; justify-content: flex-end; }
  .save-btn {
    background: var(--primary); color: white; border: none; padding: 1.2rem 2.5rem;
    border-radius: 100px; font-weight: 900; cursor: pointer; display: flex; align-items: center; gap: 0.8rem;
    transition: all 0.3s cubic-bezier(0.2, 0, 0, 1);
  }
  .save-btn:hover { transform: translateY(-4px); box-shadow: 0 10px 30px var(--primary-20); }

  /* Switch UI */
  .switch {
    position: relative;
    display: inline-block;
    width: 52px;
    height: 32px;
  }
  .switch input { opacity: 0; width: 0; height: 0; }
  .slider {
    position: absolute;
    cursor: pointer;
    inset: 0;
    background-color: var(--surface-container-highest);
    transition: .4s;
    border-radius: 34px;
  }
  .slider:before {
    position: absolute;
    content: "";
    height: 24px;
    width: 24px;
    left: 4px;
    bottom: 4px;
    background-color: var(--text-muted);
    transition: .4s;
    border-radius: 50%;
  }
  input:checked + .slider { background-color: var(--primary); }
  input:checked + .slider:before {
    transform: translateX(20px);
    background-color: white;
  }

  @media (max-width: 768px) {
    .settings-card { padding: 1.5rem; border-radius: 24px; gap: 2rem; }
    h1 { font-size: 2rem; }
    .save-btn { width: 100%; justify-content: center; }
    .add-key-form { flex-direction: column; }
    .add-btn { padding: 1rem; }
  }

  /* Success Key Modal styles */
  .new-key-display { display: flex; flex-direction: column; gap: 1.5rem; text-align: center; padding: 1rem 0; }
  .key-box {
    background: var(--surface-container-low);
    padding: 1.2rem;
    border-radius: 20px;
    border: 1px solid var(--primary-30);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
  }
  .key-box code { color: var(--primary); font-family: monospace; font-weight: 800; font-size: 1.1rem; word-break: break-all; }
  .btn-primary-rounded {
    background: var(--primary); color: white; border: none; padding: 1rem;
    border-radius: 100px; font-weight: 800; cursor: pointer; transition: all 0.2s;
  }
  .btn-primary-rounded:hover { filter: brightness(1.1); transform: translateY(-2px); }
</style>
