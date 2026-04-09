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
      <div>
        <h2>Bulk Edit: {selectedTracks.length} tracks</h2>
        {#if processingStatus}
          <span class="status-badge">{processingStatus}</span>
        {/if}
      </div>
      <button class="close-btn" onclick={closeModal}><Icon name="x" size={20} color="#94a3b8" /></button>
    </div>

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
    background: #0f111a;
    padding: 2.5rem;
    border-radius: 40px;
    width: 95%;
    max-width: 1000px;
    height: 80vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 40px 100px rgba(0,0,0,0.8);
    border: 1px solid rgba(255,255,255,0.1);
    overflow: hidden;
  }

  .relative { position: relative; }

  .suggestions {
    position: absolute;
    top: 100%; left: 0; width: 100%;
    background: #1e293b;
    border: 1px solid rgba(255,255,255,0.1);
    border-radius: 12px;
    z-index: 10;
    margin-top: 4px;
    max-height: 150px;
    overflow-y: auto;
  }

  .suggestions button {
    display: block; width: 100%; text-align: left;
    padding: 0.6rem 1rem; background: transparent;
    border: none; color: white; cursor: pointer;
  }

  .suggestions button:hover { background: rgba(255, 51, 102, 0.2); }

  .modal-header { display: flex; justify-content: space-between; margin-bottom: 2rem; }

  .status-badge {
    background: rgba(255, 51, 102, 0.2);
    color: #ff3366; padding: 0.3rem 0.8rem;
    border-radius: 100px; font-size: 0.8rem; font-weight: 700;
  }

  .bulk-layout { display: grid; grid-template-columns: 300px 1fr; gap: 2.5rem; flex: 1; overflow: hidden; }

  .bulk-sidebar { display: flex; flex-direction: column; gap: 1.5rem; }

  .hint { font-size: 0.8rem; color: #64748b; margin-top: -1rem; }

  .tag-form { display: flex; flex-direction: column; gap: 1.2rem; }

  .tag-form label { display: flex; flex-direction: column; gap: 0.4rem; color: #94a3b8; font-size: 0.9rem; }

  .tag-form input {
    background: #1e293b; border: 1px solid rgba(255,255,255,0.05);
    color: white; padding: 0.7rem 1rem; border-radius: 16px; outline: none;
  }

  .bulk-actions { display: grid; grid-template-columns: 1fr 1fr; gap: 0.8rem; margin-top: 1rem; }

  .tool-btn {
    background: rgba(255, 255, 255, 0.05); color: white;
    border: 1px solid rgba(255, 255, 255, 0.1); padding: 0.8rem;
    border-radius: 16px; font-size: 0.8rem; font-weight: 600; cursor: pointer;
  }

  .tracks-list-area {
    background: rgba(0,0,0,0.2); border-radius: 24px;
    overflow-y: auto; padding: 1rem; border: 1px solid rgba(255,255,255,0.03);
    min-height: 0;
  }

  .mini-tracks-table { width: 100%; border-collapse: collapse; font-size: 0.85rem; }
  .mini-tracks-table th { text-align: left; color: #64748b; padding: 0.8rem; }
  .mini-tracks-table td { padding: 0.8rem; border-bottom: 1px solid rgba(255,255,255,0.02); color: #cbd5e1; }

  .footer-actions {
    display: flex; justify-content: flex-end; gap: 1.5rem;
    margin-top: 2rem; padding-top: 1.5rem; border-top: 1px solid rgba(255,255,255,0.05);
  }

  .save-btn {
    background: #ff3366; color: white; padding: 1rem 2.5rem;
    border-radius: 100px; font-weight: 700; border: none; cursor: pointer;
  }

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
