<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import { audioQuality } from './stores.js';
  import { generatePalette, applyTheme } from './theme.js';
  
  let config = { musicPath: '', autoRename: false, renamePattern: '', themeColor: '#ff3366' };
  let loading = false;
  let saveSuccess = false;

  onMount(async () => {
    const res = await fetch('/api/config');
    config = await res.json();
    if (config.themeColor) {
        applyTheme(generatePalette(config.themeColor));
    }
  });

  function updateTheme() {
    const palette = generatePalette(config.themeColor);
    applyTheme(palette);
  }

  async function saveConfig() {
    loading = true;
    try {
      const res = await fetch('/api/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          musicPath: config.musicPath,
          autoRename: config.autoRename,
          renamePattern: config.renamePattern,
          themeColor: config.themeColor
        })
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
        <h3>Theme Seed Color</h3>
        <p>Choose a color to generate a custom Material Design 3 palette (Expressive).</p>
      </div>
      <div class="color-picker-wrap">
          <input type="color" bind:value={config.themeColor} on:input={updateTheme} />
          <span class="hex-val">{config.themeColor}</span>
      </div>
    </section>

    <section class="setting-item">
      <div class="info">
        <h3>Library Path</h3>
        <p>The root directory where Sgound scans for audio files.</p>
      </div>
      <input type="text" bind:value={config.musicPath} />
    </section>

    <section class="setting-item">
      <div class="info">
        <h3>Organization Pattern</h3>
        <p>Define folder structure and filenames. Available tags:</p>
        <div class="tag-cloud">
          <span>{"{artist}"}</span> <span>{"{album}"}</span> <span>{"{title}"}</span> 
          <span>{"{track}"}</span> <span>{"{year}"}</span> <span>{"{genre}"}</span>
          <span>{"{format}"}</span> <span>{"{bitrate}"}</span> <span>{"{disc}"}</span>
        </div>
      </div>
      <input type="text" class="pattern-input" bind:value={config.renamePattern} />
    </section>

    <section class="setting-item">
      <div class="info">
        <h3>Auto-Organize Files</h3>
        <p>Automatically move files according to the pattern when tags are updated.</p>
      </div>
      <label class="switch">
        <input type="checkbox" bind:checked={config.autoRename}>
        <span class="slider"></span>
      </label>
    </section>

    <section class="setting-item">
      <div class="info">
        <h3>Audio Quality</h3>
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
      <button class="save-btn" on:click={saveConfig} disabled={loading}>
        {loading ? 'Saving...' : saveSuccess ? 'Saved' : 'Save Configuration'}
        {#if saveSuccess}<Icon name="check" size={16} color="#10b981" />{/if}
      </button>
    </div>
  </div>
</div>

<style>
  .settings-view { max-width: 800px; padding: 1rem 0; animation: fadeIn 0.3s ease; }
  h1 { font-size: 2.5rem; margin-bottom: 2rem; }

  .settings-card {
    background: #161b2c; border-radius: var(--radius); padding: 2.5rem;
    border: 1px solid rgba(255,255,255,0.05); display: flex; flex-direction: column; gap: 2.5rem;
  }

  .setting-item { display: flex; justify-content: space-between; align-items: center; gap: 2rem; }
  
  .info h3 { margin: 0 0 0.5rem 0; font-size: 1.2rem; font-family: var(--font-headline); }
  .info p { margin: 0; color: #64748b; font-size: 0.9rem; line-height: 1.5; }

  .color-picker-wrap {
      display: flex;
      align-items: center;
      gap: 1rem;
      background: #1e293b;
      padding: 0.5rem 1rem;
      border-radius: 100px;
      border: 1px solid rgba(255,255,255,0.1);
  }

  input[type="color"] {
      appearance: none;
      -webkit-appearance: none;
      border: none;
      width: 32px;
      height: 32px;
      cursor: pointer;
      background: none;
      padding: 0;
  }
  input[type="color"]::-webkit-color-swatch-wrapper { padding: 0; }
  input[type="color"]::-webkit-color-swatch { border: none; border-radius: 50%; }

  .hex-val { font-family: monospace; font-weight: 700; color: var(--primary); }

  input[type="text"], .select-input {
    background: #1e293b; border: 1px solid rgba(255,255,255,0.1);
    color: white; padding: 0.8rem 1.2rem; border-radius: calc(var(--radius) / 2.5); width: 300px; outline: none;
  }

  .select-input { cursor: pointer; appearance: none; background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='24' height='24' viewBox='0 0 24 24' fill='none' stroke='white' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'%3E%3C/polyline%3E%3C/svg%3E"); background-repeat: no-repeat; background-position: right 1rem center; background-size: 1.2rem; }

  .pattern-input { font-family: monospace; color: var(--primary) !important; }

  .tag-cloud { display: flex; flex-wrap: wrap; gap: 0.5rem; margin-top: 0.5rem; }
  .tag-cloud span { 
    background: rgba(255,255,255,0.05); padding: 0.2rem 0.5rem; 
    border-radius: 6px; font-size: 0.75rem; color: #cbd5e1; font-family: monospace;
  }

  /* Switch Style MD3 */
  .switch { position: relative; display: inline-block; width: 52px; height: 32px; }
  .switch input { opacity: 0; width: 0; height: 0; }
  .slider {
    position: absolute; cursor: pointer; top: 0; left: 0; right: 0; bottom: 0;
    background-color: #334155; transition: .4s; border-radius: 32px;
  }
  .slider:before {
    position: absolute; content: ""; height: 24px; width: 24px; left: 4px; bottom: 4px;
    background-color: white; transition: .4s; border-radius: 50%;
  }
  input:checked + .slider { background-color: var(--primary); }
  input:checked + .slider:before { transform: translateX(20px); }

  .save-btn {
    background: var(--primary); color: white; border: none; padding: 1rem 2.5rem;
    border-radius: 100px; font-weight: 700; cursor: pointer; align-self: flex-end;
  }
</style>
