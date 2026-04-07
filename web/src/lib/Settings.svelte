<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import { audioQuality } from './stores.js';
  import { generatePalette, applyTheme } from './theme.js';
  
  let config = $state({ musicPath: '', autoRename: false, renamePattern: '', themeColor: '#ff3366', replayGainEnabled: false });
  let loading = $state(false);
  let saveSuccess = $state(false);

  // Derivamos la paleta automáticamente de la semilla
  let palette = $derived(generatePalette(config.themeColor));

  onMount(async () => {
    const res = await fetch('/api/config');
    const data = await res.json();
    config.musicPath = data.musicPath;
    config.autoRename = data.autoRename;
    config.themeColor = data.themeColor || '#ff3366';
    config.replayGainEnabled = data.replayGainEnabled;
    applyTheme(palette);
    fetchAPIKeys();
  });

  let apiKeys = $state([]);
  let newKeyLabel = $state('');
  let newKeyPerms = $state('r');
  let myPerms = $derived(localStorage.getItem('permissions') || 'r');

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
      newKeyLabel = '';
      fetchAPIKeys();
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

  async function saveConfig() {
    loading = true;
    try {
      const res = await fetch('/api/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config)
      });
      if (res.ok) {
        saveSuccess = true;
        setTimeout(() => saveSuccess = false, 2000);
      }
    } catch (e) { alert("Failed to save config"); }
    finally { loading = false; }
  }
</script>

<div class="settings-view">
  <h1>Server Settings</h1>
  
  <div class="settings-card">
    <section class="setting-item">
      <div class="info">
        <div class="title-with-help">
          <h3>Theme Seed Color</h3>
          <div class="tooltip-wrap">
            <Icon name="info" size={14} color="var(--text-muted)" />
            <span class="tooltip">Define el color base para generar la paleta Material Design 3.</span>
          </div>
        </div>
        <p>Choose a color to generate a custom Material Design 3 palette (Expressive).</p>
      </div>
      <div class="color-section">
          <div class="color-picker-custom">
              <label class="color-swatch" style="background-color: {config.themeColor}">
                <input type="color" bind:value={config.themeColor} />
              </label>
              <span class="hex-val">{config.themeColor}</span>
          </div>
          
          <div class="palette-preview">
              <div class="swatch-group">
                  <div class="mini-swatch" style="background-color: {palette.primary}"></div>
                  <span>Primary</span>
              </div>
              <div class="swatch-group">
                  <div class="mini-swatch" style="background-color: {palette.secondary}"></div>
                  <span>Secondary</span>
              </div>
              <div class="swatch-group">
                  <div class="mini-swatch" style="background-color: {palette.tertiary}"></div>
                  <span>Tertiary</span>
              </div>
          </div>
      </div>
    </section>

    <section class="setting-item">
      <div class="info">
        <div class="title-with-help">
          <h3>Library Path</h3>
          <div class="tooltip-wrap">
            <Icon name="info" size={14} color="var(--text-muted)" />
            <span class="tooltip">Ruta absoluta o relativa donde se encuentra tu música.</span>
          </div>
        </div>
        <p>The root directory where Sgound scans for audio files.</p>
      </div>
      <input type="text" bind:value={config.musicPath} placeholder="/home/user/music" />
    </section>

    <section class="setting-item">
      <div class="info">
        <div class="title-with-help">
          <h3>Organization Pattern</h3>
          <div class="tooltip-wrap">
            <Icon name="info" size={14} color="var(--text-muted)" />
            <span class="tooltip">Usa etiquetas como {"{artist}"} para definir cómo se organizan los archivos físicamente.</span>
          </div>
        </div>
        <p>Define folder structure and filenames. Available tags:</p>
        <div class="tag-cloud">
          <span>{"{artist}"}</span> <span>{"{album}"}</span> <span>{"{title}"}</span> 
          <span>{"{track}"}</span> <span>{"{year}"}</span> <span>{"{genre}"}</span>
          <span>{"{format}"}</span> <span>{"{bitrate}"}</span> <span>{"{disc}"}</span>
        </div>
      </div>
      <input type="text" class="pattern-input" bind:value={config.renamePattern} placeholder={`{artist}/{album}/{title}`} />
    </section>

    <section class="setting-item">
      <div class="info">
        <div class="title-with-help">
          <h3>Auto-Organize Files</h3>
          <div class="tooltip-wrap">
            <Icon name="info" size={14} color="var(--text-muted)" />
            <span class="tooltip">Si se activa, moverá los archivos físicamente al editar los tags. Cuidado.</span>
          </div>
        </div>
        <p>Automatically move files according to the pattern when tags are updated.</p>
      </div>
      <label class="switch">
        <input type="checkbox" bind:checked={config.autoRename}>
        <span class="slider"></span>
      </label>
    </section>

    <section class="setting-item">
      <div class="info">
        <div class="title-with-help">
          <h3>ReplayGain (Beta)</h3>
          <div class="tooltip-wrap">
            <Icon name="info" size={14} color="var(--text-muted)" />
            <span class="tooltip">Si se activa, el servidor procesará las etiquetas de ganancia para normalizar el volumen.</span>
          </div>
        </div>
        <p>Normalize loudness levels across different tracks automatically.</p>
      </div>
      <label class="switch">
        <input type="checkbox" bind:checked={config.replayGainEnabled}>
        <span class="slider"></span>
      </label>
    </section>

    <section class="setting-item">
      <div class="info">
        <div class="title-with-help">
          <h3>Audio Quality</h3>
          <div class="tooltip-wrap">
            <Icon name="info" size={14} color="var(--text-muted)" />
            <span class="tooltip">Calidad de salida. 'Original' no consume CPU, las demás usan FFmpeg.</span>
          </div>
        </div>
        <p>Choose the bitrate for streaming. Transcoding will be used for lower bitrates.</p>
      </div>
      <select bind:value={$audioQuality} class="select-input">
        <option value="original">Original (No transcoding)</option>
        <option value="320k">High (320 kbps MP3)</option>
        <option value="192k">Medium (192 kbps MP3)</option>
        <option value="128k">Low (128 kbps MP3)</option>
        <option value="64k">Mobile/Data Saver (64 kbps MP3)</option>
      </select>
    </section>

    <div class="actions">
      <button class="save-btn" onclick={saveConfig} disabled={loading}>
        {loading ? 'Saving...' : saveSuccess ? 'Saved' : 'Save Configuration'}
        {#if saveSuccess}<Icon name="check" size={16} color="#10b981" />{/if}
      </button>
    </div>
  </div>

  <div class="settings-card" style="margin-top: 2rem;">
    <h2>My API Keys</h2>
    <p class="section-desc">Utiliza estas llaves para conectar aplicaciones externas como Dsub, Ample o Symfonium.</p>
    
    <div class="api-keys-list">
      {#each apiKeys as key}
        <div class="key-item">
          <div class="key-info">
            <span class="key-label">{key.Label}</span>
            <span class="badge {key.Permissions}">{key.Permissions}</span>
          </div>
          <div class="key-actions">
            <code>{key.Key}</code>
            <button class="icon-btn" onclick={() => navigator.clipboard.writeText(key.Key)} title="Copy Key">
              <Icon name="copy" size={16} />
            </button>
            <button class="icon-btn delete" onclick={() => deleteAPIKey(key.Key)} title="Revoke Key">
              <Icon name="trash" size={16} />
            </button>
          </div>
        </div>
      {/each}
    </div>

    <div class="add-key-form">
      <h3>Add New API Key</h3>
      <div class="form-row">
        <input type="text" placeholder="Device Label (e.g. My Phone)" bind:value={newKeyLabel} />
        <select bind:value={newKeyPerms} class="small-select">
          <option value="r">Read Only</option>
          {#if myPerms === 'rw'}
            <option value="rw">Read & Write</option>
          {/if}
        </select>
        <button class="add-btn" onclick={createAPIKey}>Generate</button>
      </div>
    </div>
  </div>
</div>

<style>
  .settings-view { max-width: 900px; padding: 1rem 0; animation: fadeIn 0.3s ease; }
  h1 { font-size: var(--font-size-h1); font-weight: var(--font-weight-h1); letter-spacing: var(--letter-spacing-h1); margin-bottom: 3rem; }

  .settings-card {
    background: var(--surface-container-high); 
    border-radius: var(--radius); 
    padding: 3rem;
    border: 1px solid var(--border-color); 
    display: flex; 
    flex-direction: column; 
    gap: 3rem;
    box-shadow: 0 20px 50px rgba(0,0,0,0.3);
  }

  .setting-item { display: flex; justify-content: space-between; align-items: center; gap: 3rem; }
  
  .info h3 { margin: 0 0 0.5rem 0; font-size: 1.3rem; font-weight: 800; font-family: var(--font-headline); color: var(--text-main); }
  .info p { margin: 0; color: var(--text-muted); font-size: 0.95rem; line-height: 1.6; }

  /* Title with Help Icon */
  .title-with-help { display: flex; align-items: center; gap: 0.6rem; }

  /* Tooltip logic */
  .tooltip-wrap { position: relative; display: flex; align-items: center; cursor: help; }
  .tooltip {
    visibility: hidden;
    position: absolute;
    bottom: 130%;
    left: 50%;
    transform: translateX(-50%);
    background: var(--surface-container-highest);
    color: var(--text-main);
    padding: 0.6rem 1rem;
    border-radius: 12px;
    font-size: 0.8rem;
    white-space: nowrap;
    z-index: 10;
    opacity: 0;
    transition: all 0.2s;
    border: 1px solid var(--border-color);
    box-shadow: 0 10px 30px rgba(0,0,0,0.4);
    font-weight: 600;
  }
  .tooltip-wrap:hover .tooltip { visibility: visible; opacity: 1; transform: translateX(-50%) translateY(-5px); }

  /* Custom Color Picker */
  .color-section { display: flex; flex-direction: column; gap: 1.2rem; align-items: flex-end; }
  
  .color-picker-custom {
      display: flex;
      align-items: center;
      gap: 1.2rem;
      background: var(--surface-container-highest);
      padding: 0.6rem 1.2rem;
      border-radius: 100px;
      border: 1px solid var(--border-color);
  }

  .color-swatch {
      width: 36px;
      height: 36px;
      border-radius: 50%;
      cursor: pointer;
      display: block;
      border: 3px solid rgba(255,255,255,0.1);
      transition: transform 0.2s;
      position: relative;
      overflow: hidden;
      box-shadow: 0 4px 8px rgba(0,0,0,0.3);
  }
  .color-swatch:hover { transform: scale(1.1); }
  .color-swatch input {
      position: absolute;
      top: -50%; left: -50%; width: 200%; height: 200%;
      cursor: pointer;
      opacity: 0;
  }

  .hex-val { font-family: monospace; font-weight: 800; color: var(--primary); font-size: 1.1rem; }

  /* Palette Preview */
  .palette-preview {
      display: flex;
      gap: 1.5rem;
      padding: 1rem 1.5rem;
      background: var(--surface-container-highest);
      border-radius: 20px;
      border: 1px solid var(--border-color);
  }

  .swatch-group { display: flex; align-items: center; gap: 0.6rem; }
  .swatch-group span { font-size: 0.75rem; font-weight: 800; color: var(--text-muted); text-transform: uppercase; letter-spacing: 1px; }

  .mini-swatch {
      width: 20px;
      height: 20px;
      border-radius: 6px;
      border: 1px solid rgba(255,255,255,0.1);
      box-shadow: 0 2px 4px rgba(0,0,0,0.2);
  }

  input[type="text"], .select-input {
    background: var(--surface-container-high); 
    border: 1px solid var(--border-color);
    color: var(--text-main); 
    padding: 0.9rem 1.5rem; 
    border-radius: 16px; 
    width: 350px; 
    outline: none;
    font-weight: 600;
    transition: all 0.2s;
  }
  input[type="text"]:focus, .select-input:focus { border-color: var(--primary); background: var(--surface); }

  .select-input { cursor: pointer; appearance: none; background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='24' height='24' viewBox='0 0 24 24' fill='none' stroke='white' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'%3E%3C/polyline%3E%3C/svg%3E"); background-repeat: no-repeat; background-position: right 1.2rem center; background-size: 1.2rem; }

  .pattern-input { font-family: monospace; color: var(--primary) !important; font-size: 1.1rem; }

  .tag-cloud { display: flex; flex-wrap: wrap; gap: 0.6rem; margin-top: 0.8rem; }
  .tag-cloud span { 
    background: var(--surface-container-highest); 
    padding: 0.3rem 0.7rem; 
    border-radius: 8px; font-size: 0.8rem; color: var(--text-main); 
    font-family: monospace;
    border: 1px solid var(--border-color);
  }

  /* Switch Style MD3 */
  .switch { position: relative; display: inline-block; width: 56px; height: 32px; }
  .switch input { opacity: 0; width: 0; height: 0; }
  .slider {
    position: absolute; cursor: pointer; top: 0; left: 0; right: 0; bottom: 0;
    background-color: var(--surface-container-highest); transition: .4s; border-radius: 32px;
    border: 1px solid var(--border-color);
  }
  .slider:before {
    position: absolute; content: ""; height: 22px; width: 22px; left: 4px; bottom: 4px;
    background-color: var(--text-muted); transition: .4s; border-radius: 50%;
  }
  input:checked + .slider { background-color: var(--primary); border-color: var(--primary); }
  input:checked + .slider:before { transform: translateX(24px); background-color: white; }

  .actions { display: flex; justify-content: flex-end; margin-top: 1rem; }

  .save-btn {
    background: var(--primary); 
    color: white; 
    border: none; 
    padding: 1.2rem 3rem;
    border-radius: 100px; 
    font-weight: 800; 
    cursor: pointer; 
    font-size: 1rem;
    box-shadow: 0 8px 20px var(--primary-10);
    display: flex;
    align-items: center;
    gap: 0.8rem;
    transition: all 0.3s cubic-bezier(0.2, 0, 0, 1);
  }
  .save-btn:hover { transform: translateY(-2px); box-shadow: 0 12px 25px var(--primary-10); filter: brightness(1.1); }
  .save-btn:active { transform: scale(0.98); }
  /* API Key Styles */
  .section-desc { color: var(--text-muted); margin-bottom: 2rem; font-size: 0.95rem; }
  .api-keys-list { display: flex; flex-direction: column; gap: 1rem; margin-bottom: 3rem; }
  .key-item { 
    display: flex; justify-content: space-between; align-items: center; 
    padding: 1.2rem 2rem; background: rgba(255,255,255,0.02); 
    border-radius: 20px; border: 1px solid var(--border-color);
  }
  .key-info { display: flex; align-items: center; gap: 1.5rem; }
  .key-label { font-weight: 800; color: var(--text-main); font-size: 1rem; }
  .key-actions { display: flex; align-items: center; gap: 1rem; }
  .key-actions code { background: rgba(0,0,0,0.3); padding: 0.4rem 1rem; border-radius: 8px; font-family: monospace; color: var(--primary); }
  
  .badge { padding: 0.2rem 0.6rem; border-radius: 6px; font-size: 0.7rem; font-weight: 950; text-transform: uppercase; }
  .badge.rw { background: var(--primary-10); color: var(--primary); border: 1px solid var(--primary); }
  .badge.r { background: rgba(255,255,255,0.05); color: var(--text-muted); border: 1px solid var(--border-color); }

  .icon-btn.delete:hover { background: rgba(239, 68, 68, 0.1); color: #f87171; }

  .add-key-form { background: var(--surface-container-highest); padding: 2rem; border-radius: 24px; border: 1px solid var(--border-color); }
  .small-select { 
    background: var(--surface-container-high); border: 1px solid var(--border-color); 
    color: var(--text-main); padding: 0.8rem 1.2rem; border-radius: 12px; outline: none; font-weight: 700;
  }
</style>
