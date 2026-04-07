<script>
  import { onMount } from 'svelte';
  
  let config = { musicPath: '', autoRename: false, renamePattern: '' };
  let loading = false;
  let saveSuccess = false;

  onMount(async () => {
    const res = await fetch('/api/config');
    config = await res.json();
  });

  async function saveConfig() {
    loading = true;
    try {
      const res = await fetch('/api/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          musicPath: config.musicPath,
          autoRename: config.autoRename,
          renamePattern: config.renamePattern
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

    <div class="actions">
      <button class="save-btn" on:click={saveConfig} disabled={loading}>
        {loading ? 'Saving...' : saveSuccess ? 'Saved ✓' : 'Save Configuration'}
      </button>
    </div>
  </div>
</div>

<style>
  .settings-view { max-width: 800px; padding: 1rem 0; animation: fadeIn 0.3s ease; }
  h1 { font-size: 2.5rem; margin-bottom: 2rem; }

  .settings-card {
    background: #161b2c; border-radius: 32px; padding: 2.5rem;
    border: 1px solid rgba(255,255,255,0.05); display: flex; flex-direction: column; gap: 2.5rem;
  }

  .setting-item { display: flex; justify-content: space-between; align-items: center; gap: 2rem; }
  
  .info h3 { margin: 0 0 0.5rem 0; font-size: 1.2rem; }
  .info p { margin: 0; color: #64748b; font-size: 0.9rem; line-height: 1.5; }

  input[type="text"] {
    background: #1e293b; border: 1px solid rgba(255,255,255,0.1);
    color: white; padding: 0.8rem 1.2rem; border-radius: 12px; width: 300px; outline: none;
  }

  .pattern-input { font-family: monospace; color: #ff3366 !important; }

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
  input:checked + .slider { background-color: #ff3366; }
  input:checked + .slider:before { transform: translateX(20px); }

  .save-btn {
    background: #ff3366; color: white; border: none; padding: 1rem 2.5rem;
    border-radius: 100px; font-weight: 700; cursor: pointer; align-self: flex-end;
  }
</style>
