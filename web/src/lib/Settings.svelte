<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import { audioQuality, showToast } from './stores.js';
  import { generatePalette, applyTheme } from './theme.js';
  import Modal from './Modal.svelte';
  
  let config = $state({ musicPath: '', autoRename: false, renamePattern: '', themeColor: '#ff3366', replayGainEnabled: false });
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
    config.replayGainEnabled = data.replayGainEnabled;
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

  // Efecto para aplicar el tema cuando cambia la paleta (Svelte 5)
  $effect(() => {
    applyTheme(palette);
  });

  let organizeStatus = $state({ isRunning: false, total: 0, processed: 0, current: '' });

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

  async function startOrganize() {
    if (!confirm("This will move ALL files in your library to match the pattern. Are you sure?")) return;
    
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

      <div class="settings-card" style="margin-top: 2rem;">
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
            <p>Use placeholders: <b>&#123;artist&#125;, &#123;album&#125;, &#123;title&#125;, &#123;year&#125;, &#123;genre&#125;, &#123;track&#125;, &#123;bitrate&#125;</b></p>
          </div>
          <input type="text" bind:value={config.renamePattern} placeholder="{artist}/{album}/{track} - {title}" />
          
          {#if organizeStatus.isRunning}
            <div class="organize-progress-wrap">
              <div class="progress-bar-bg">
                 <div class="progress-bar-fill" style="width: {(organizeStatus.processed / organizeStatus.total) * 100}%"></div>
              </div>
              <div class="progress-stats">
                 <span class="p-count">{organizeStatus.processed} / {organizeStatus.total} tracks</span>
                 <span class="p-current">Moving: {organizeStatus.current}</span>
              </div>
            </div>
          {:else}
            <button class="scan-btn" onclick={startOrganize}>
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
    </div>

    <div class="settings-column">
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

      <div class="settings-card" style="margin-top: 2rem;">
        <h2>API Keys</h2>
        <p class="section-desc">Tokens for external apps and mobile clients.</p>
        
        <div class="api-keys-list">
          {#each apiKeys as key}
            <div class="key-item">
              <div class="key-info">
                <span class="key-label">{key.label}</span>
                <span class="badge {key.permissions}">{key.permissions}</span>
              </div>
              <div class="key-actions">
                <button class="icon-btn" onclick={() => navigator.clipboard.writeText(key.key)} title="Copy"><Icon name="copy" size={16} /></button>
                <button class="icon-btn delete" onclick={() => deleteAPIKey(key.key)}><Icon name="trash" size={16} /></button>
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
        <button class="icon-btn" onclick={() => { navigator.clipboard.writeText(newlyCreatedKey); showToast("Copied to clipboard", "info"); }}>
          <Icon name="copy" size={20} />
        </button>
      </div>
      <button class="btn-primary-rounded" onclick={() => showKeyModal = false}>Got it</button>
    </div>
  </Modal>
</div>

<style>
  .settings-view { width: 100%; padding: 0 0 4rem 0; animation: fadeIn 0.4s ease-out; }
  h1 { font-size: 2.5rem; font-weight: 900; letter-spacing: -1.5px; margin: 0 0 2.5rem 0; color: var(--text-main); }
  
  .settings-grid {
    display: grid;
    grid-template-columns: 1fr;
    gap: 2rem;
    align-items: start;
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
    display: flex; justify-content: space-between; align-items: center; 
    padding: 1.2rem; background: var(--surface-container-low); border-radius: 20px; border: 1px solid var(--border-color);
  }
  .key-info { display: flex; flex-direction: column; gap: 0.4rem; }
  .key-label { font-weight: 800; color: var(--text-main); }
  .badge { padding: 0.2rem 0.5rem; border-radius: 6px; font-size: 0.7rem; font-weight: 900; width: fit-content; }
  .badge.rw { background: var(--primary-10); color: var(--primary); }
  
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
