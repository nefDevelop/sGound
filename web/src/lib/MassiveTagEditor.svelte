<script>
  import { createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';
  import { showToast } from './stores.js';
  
  let { 
    isOpen = $bindable(false), 
    selectedTracks = $bindable([]), 
    onclose = null 
  } = $props();
  
  let loading = $state(false);
  let saveSuccess = $state(false);
  let processingStatus = $state("");
  
  let suggestions = $state([]);
  let activeSuggestionField = $state("");

  // Datos globales que se aplicarán a todos
  let bulkData = $state({
    artist: '',
    album: '',
    genre: '',
    year: '',
    composer: ''
  });

  async function fetchSuggestions(field, value) {
    if (value.length < 1) {
      suggestions = [];
      return;
    }
    try {
      const res = await fetch(`/api/autocomplete?field=${field}&q=${encodeURIComponent(value)}`);
      suggestions = await res.json();
      activeSuggestionField = field;
    } catch (e) { console.error(e); }
  }

  function selectSuggestion(val) {
    bulkData[activeSuggestionField] = val;
    suggestions = [];
  }

  async function handleBulkSave() {
    if (selectedTracks.length === 0) return;
    loading = true;
    processingStatus = "Saving all tracks...";
    
    try {
      const promises = selectedTracks.map(track => {
        const payload = { ...track };
        if (bulkData.artist) payload.artist = bulkData.artist;
        if (bulkData.album) payload.album = bulkData.album;
        if (bulkData.genre) payload.genre = bulkData.genre;
        if (bulkData.year) payload.year = parseInt(bulkData.year);
        if (bulkData.composer) payload.composer = bulkData.composer;

        const p1 = fetch(`/api/tracks/${track.id}/tags`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });

        const p2 = track.lyrics ? fetch(`/api/lyrics/${track.id}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ lyrics: track.lyrics })
        }) : Promise.resolve(null);

        return Promise.all([p1, p2]);
      });

      await Promise.all(promises);
      
      saveSuccess = true;
      processingStatus = "Success!";
      setTimeout(() => {
        saveSuccess = false;
        closeModal();
      }, 1500);
      
    } catch (e) {
      showToast("Error saving bulk tags: " + e.message, "error");
    } finally {
      loading = false;
    }
  }

  async function autoSearchAll() {
    loading = true;
    let count = 0;
    for (let track of selectedTracks) {
      processingStatus = `Searching metadata ${++count}/${selectedTracks.length}...`;
      try {
        const res = await fetch(`/api/metadata/search?artist=${encodeURIComponent(track.artist)}&title=${encodeURIComponent(track.title)}`);
        if (res.ok) {
          const results = await res.json();
          if (results.length > 0) {
            const data = results[0];
            track.album = data.album || track.album;
            track.year = data.year || track.year;
            track.genre = data.genre || track.genre;
            track.trackNum = data.trackNum || track.trackNum;
          }
        }
      } catch (e) { console.error(e); }
    }
    processingStatus = "Auto-search complete";
    loading = false;
  }

  async function downloadAllLyrics() {
    loading = true;
    let count = 0;
    for (let track of selectedTracks) {
      processingStatus = `Downloading lyrics ${++count}/${selectedTracks.length}...`;
      try {
        const res = await fetch(`/api/lyrics/${track.id}`);
        const data = await res.json();
        if (data.lyrics) track.lyrics = data.lyrics;
      } catch (e) { console.error(e); }
    }
    processingStatus = "Lyrics download complete";
    loading = false;
  }

  function closeModal() {
    isOpen = false;
    processingStatus = "";
    suggestions = [];
    if (onclose) onclose();
  }
</script>

{#if isOpen}
<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="modal-backdrop" onclick={closeModal}>
    <div class="modal glass" onclick={(e) => e.stopPropagation()}>
      <div class="modal-header">
        <div class="header-info">
          <h2>Bulk Edit Tags</h2>
          <p class="subtitle">{selectedTracks.length} tracks selected</p>
        </div>
        <button class="close-btn" onclick={closeModal}><Icon name="x" size={20} color="#94a3b8" /></button>
      </div>

      {#if processingStatus}
        <div class="global-status">
          <div class="status-content">
            <div class="spinner"></div>
            <span>{processingStatus}</span>
          </div>
        </div>
      {/if}

    <div class="bulk-layout">
      <aside class="bulk-sidebar">
        <h3>Common Fields</h3>
        <p class="hint">Fields left empty won't be changed.</p>
        
        <div class="tag-form">
          <label class="relative">
            <span>Artist</span>
            <input type="text" bind:value={bulkData.artist} oninput={(e) => fetchSuggestions('artist', e.currentTarget.value)} />
            {#if suggestions.length > 0 && activeSuggestionField === 'artist'}
              <div class="suggestions">
                {#each suggestions as s}
                  <button type="button" onclick={() => selectSuggestion(s)}>{s}</button>
                {/each}
              </div>
            {/if}
          </label>

          <label class="relative">
            <span>Album</span>
            <input type="text" bind:value={bulkData.album} oninput={(e) => fetchSuggestions('album', e.currentTarget.value)} />
            {#if suggestions.length > 0 && activeSuggestionField === 'album'}
              <div class="suggestions">
                {#each suggestions as s}
                  <button type="button" onclick={() => selectSuggestion(s)}>{s}</button>
                {/each}
              </div>
            {/if}
          </label>

          <label class="relative">
            <span>Genre</span>
            <input type="text" bind:value={bulkData.genre} oninput={(e) => fetchSuggestions('genre', e.currentTarget.value)} />
            {#if suggestions.length > 0 && activeSuggestionField === 'genre'}
              <div class="suggestions">
                {#each suggestions as s}
                  <button type="button" onclick={() => selectSuggestion(s)}>{s}</button>
                {/each}
              </div>
            {/if}
          </label>

          <label><span>Year</span><input type="number" bind:value={bulkData.year} /></label>
          
          <div class="bulk-actions">
            <button class="tool-btn" onclick={autoSearchAll} disabled={loading}><Icon name="zap" size={14} /> Auto-Tag</button>
            <button class="tool-btn" onclick={downloadAllLyrics} disabled={loading}><Icon name="cloud" size={14} /> Lyrics</button>
          </div>
        </div>
      </aside>

      <section class="tracks-list-area">
        <table class="mini-tracks-table">
          <thead>
            <tr>
              <th>#</th>
              <th>Title</th>
              <th>Album</th>
              <th>Lyrics?</th>
            </tr>
          </thead>
          <tbody>
            {#each selectedTracks as track}
              <tr>
                <td>{track.trackNum || '-'}</td>
                <td class="bold">{track.title}</td>
                <td class="muted">{track.album}</td>
                <td>
                  {#if track.lyrics}
                    <Icon name="checkCircle" size={14} color="#10b981" />
                  {:else}
                    <Icon name="xCircle" size={14} color="#f87171" />
                  {/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </section>
    </div>

    <div class="footer-actions">
      <button type="button" class="cancel-btn" onclick={closeModal}>Cancel</button>
      <button type="button" class="save-btn" onclick={handleBulkSave} disabled={loading}>
        {loading ? 'Processing...' : saveSuccess ? 'Saved' : 'Apply Changes'}
        {#if saveSuccess}<Icon name="check" size={16} color="#10b981" />{/if}
      </button>
    </div>
  </div>
</div>
{/if}

<style>
  .modal-backdrop {
    position: fixed;
    top: 0; left: 0; width: 100%; height: 100%;
    background: rgba(0,0,0,0.7);
    backdrop-filter: blur(12px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 5000;
  }

  .modal {
    background: var(--surface-container-highest);
    padding: 0;
    border-radius: 40px;
    width: 95%;
    max-width: 1100px;
    height: 85vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 40px 120px rgba(0,0,0,0.6);
    border: 1px solid var(--border-color);
    overflow: hidden;
  }

  .relative { position: relative; }

  .suggestions {
    position: absolute;
    top: 100%; left: 0; width: 100%;
    background: var(--surface-container-highest);
    border: 1px solid var(--border-color);
    border-radius: 12px;
    z-index: 10;
    margin-top: 4px;
    max-height: 150px;
    overflow-y: auto;
    box-shadow: 0 10px 25px rgba(0,0,0,0.5);
  }

  .suggestions button {
    display: block; width: 100%; text-align: left;
    padding: 0.8rem 1rem; background: transparent;
    border: none; color: var(--text-main); cursor: pointer;
  }

  .suggestions button:hover { background: var(--primary-10); color: var(--primary); }

  .modal-header { 
    display: flex; 
    justify-content: space-between; 
    align-items: center;
    padding: 1.5rem 2.5rem;
    border-bottom: 1px solid var(--border-color);
  }
  .header-info h2 { margin: 0; font-size: 1.25rem; font-weight: 900; color: white; }
  .header-info .subtitle { margin: 0.2rem 0 0; font-size: 0.85rem; color: var(--text-muted); font-weight: 600; }

  .global-status {
    background: var(--primary-10);
    padding: 0.8rem 2.5rem;
    border-bottom: 1px solid var(--primary-20);
  }
  .status-content { display: flex; align-items: center; gap: 1rem; color: var(--primary); font-weight: 700; font-size: 0.9rem; }
  
  .spinner {
    width: 18px; height: 18px; border: 2px solid var(--primary-30); border-top-color: var(--primary);
    border-radius: 50%; animation: spin 1s linear infinite;
  }
  @keyframes spin { to { transform: rotate(360deg); } }

  .bulk-layout { display: grid; grid-template-columns: 340px 1fr; gap: 0; flex: 1; overflow: hidden; }

  .bulk-sidebar { 
    display: flex; 
    flex-direction: column; 
    gap: 1.5rem; 
    padding: 2.5rem;
    background: rgba(0,0,0,0.15);
    border-right: 1px solid var(--border-color);
  }

  .bulk-sidebar h3 { margin: 0; font-size: 1rem; color: white; font-weight: 800; }
  .hint { font-size: 0.8rem; color: var(--text-muted); margin-top: -1rem; font-weight: 600; }

  .tag-form { display: flex; flex-direction: column; gap: 1.5rem; }

  .tag-form label { display: flex; flex-direction: column; gap: 0.5rem; color: var(--text-muted); font-size: 0.85rem; font-weight: 700; }

  .tag-form input {
    background: var(--surface-container); 
    border: 1px solid var(--border-color);
    color: white; 
    padding: 0.8rem 1.2rem; 
    border-radius: 14px; 
    outline: none;
    transition: all 0.2s;
  }
  .tag-form input:focus { border-color: var(--primary); background: var(--surface); }

  .bulk-actions { display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; margin-top: 0.5rem; }

  .tool-btn {
    background: var(--surface-container-high); 
    color: var(--text-main);
    border: 1px solid var(--border-color); 
    padding: 0.8rem;
    border-radius: 16px; 
    font-size: 0.8rem; 
    font-weight: 700; 
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    transition: all 0.2s;
  }
  .tool-btn:hover { background: var(--surface-container-highest); border-color: var(--primary-30); color: var(--primary); }

  .tracks-list-area {
    overflow-y: auto; 
    padding: 2.5rem;
    min-height: 0;
  }

  .mini-tracks-table { width: 100%; border-collapse: separate; border-spacing: 0 4px; font-size: 0.9rem; }
  .mini-tracks-table th { text-align: left; color: var(--text-muted); padding: 0.8rem 1rem; font-weight: 800; text-transform: uppercase; font-size: 0.7rem; letter-spacing: 1px; }
  .mini-tracks-table td { padding: 0.8rem 1rem; border-top: 1px solid transparent; border-bottom: 1px solid transparent; color: var(--text-main); }
  
  .mini-tracks-table tr:hover td { background: var(--surface-container-high); }
  .mini-tracks-table td:first-child { border-top-left-radius: 12px; border-bottom-left-radius: 12px; }
  .mini-tracks-table td:last-child { border-top-right-radius: 12px; border-bottom-right-radius: 12px; }
  
  .mini-tracks-table .bold { font-weight: 700; color: white; }
  .mini-tracks-table .muted { color: var(--text-muted); opacity: 0.8; }

  .footer-actions {
    display: flex; 
    justify-content: flex-end; 
    gap: 1.5rem;
    padding: 2rem 2.5rem; 
    border-top: 1px solid var(--border-color);
    background: rgba(0,0,0,0.1);
  }

  .save-btn {
    background: var(--primary); 
    color: white; 
    padding: 1rem 3rem;
    border-radius: 100px; 
    font-weight: 800; 
    border: none; 
    cursor: pointer;
    box-shadow: 0 10px 30px var(--primary-20);
    transition: all 0.2s;
    display: flex;
    align-items: center;
    gap: 0.8rem;
  }
  .save-btn:hover { transform: translateY(-3px); box-shadow: 0 15px 40px var(--primary-30); }
  .save-btn:disabled { opacity: 0.5; cursor: not-allowed; transform: none !important; }

  .cancel-btn { background: transparent; border: none; color: #64748b; font-weight: 600; cursor: pointer; }

  /* Hide number input spinners */
  input::-webkit-outer-spin-button,
  input::-webkit-inner-spin-button {
    -webkit-appearance: none;
    appearance: none;
    margin: 0;
  }
  input[type=number] {
    -moz-appearance: textfield;
    appearance: textfield;
  }
</style>
