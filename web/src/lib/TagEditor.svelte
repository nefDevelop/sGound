<script>
  import { createEventDispatcher } from 'svelte';
  
  export let isOpen = false;
  export let track = { id: '', title: '', artist: '', album: '' };
  
  const dispatch = createEventDispatcher();
  let loading = false;
  let saveSuccess = false;

  async function handleSave() {
    loading = true;
    try {
      // 1. Guardar Tags completos
      const res = await fetch(`/api/tracks/${track.id}/tags`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          title: track.title,
          artist: track.artist,
          album: track.album,
          genre: track.genre,
          year: parseInt(track.year) || 0,
          trackNum: parseInt(track.trackNum) || 0,
          discNum: parseInt(track.discNum) || 0,
          composer: track.composer,
          comment: track.comment
        })
      });

      // 2. Guardar Letras si existen
      if (track.lyrics) {
        await fetch(`/api/lyrics/${track.id}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ lyrics: track.lyrics })
        });
      }
      
      if (!res.ok) throw new Error("Failed to update tags");
      
      saveSuccess = true;
      setTimeout(() => {
        saveSuccess = false;
        closeModal();
      }, 1500);
      
    } catch (e) {
      console.error(e);
      alert("Error saving tags: " + e.message);
    } finally {
      loading = false;
    }
  }

  async function downloadLyrics() {
    try {
      const res = await fetch(`/api/lyrics/${track.id}`);
      const data = await res.json();
      if (data.lyrics) {
        track.lyrics = data.lyrics;
      } else {
        alert("Lyrics not found in internet");
      }
    } catch (e) {
      alert("Failed to fetch lyrics");
    }
  }

  let searchResults = [];
  let suggestions = [];
  let activeSuggestionField = "";

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
    track[activeSuggestionField] = val;
    suggestions = [];
  }

  async function autoSearchTags() {
    try {
      const res = await fetch(`/api/metadata/search?artist=${encodeURIComponent(track.artist)}&title=${encodeURIComponent(track.title)}`);
      if (!res.ok) throw new Error("Metadata not found");
      searchResults = await res.json();
      
      if (searchResults.length === 1) {
        applyMetadata(searchResults[0]);
      }
      // Si hay más de uno, la UI mostrará el selector automáticamente por el bloque {#if searchResults.length > 1}
    } catch (e) {
      alert("Auto search failed: " + e.message);
    }
  }

  function applyMetadata(data) {
    track.artist = data.artist || track.artist;
    track.title = data.title || track.title;
    track.album = data.album || track.album;
    track.year = data.year || track.year;
    track.genre = data.genre || track.genre;
    track.trackNum = data.trackNum || track.trackNum;
    track.discNum = data.discNum || track.discNum;
    searchResults = [];
  }
</script>

{#if isOpen}
<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="modal-backdrop" on:click={closeModal}>
  <div class="modal glass" on:click|stopPropagation>
    <div class="modal-header">
      <div class="title-with-btn">
        <h2>Edit Track Tags</h2>
        <button type="button" class="auto-btn" on:click={autoSearchTags}>⚡ Auto Search</button>
      </div>
      <button class="close-btn" on:click={closeModal}>✕</button>
    </div>

    {#if searchResults.length > 1}
      <div class="results-picker">
        <h3>Select the best match:</h3>
        <div class="results-list">
          {#each searchResults as res}
            <button class="result-item" on:click={() => applyMetadata(res)}>
              <img src={res.artwork} alt="" />
              <div class="res-info">
                <span class="res-title">{res.title}</span>
                <span class="res-sub">{res.artist} • {res.album}</span>
              </div>
            </button>
          {/each}
        </div>
        <button class="cancel-btn" on:click={() => searchResults = []}>Back to editor</button>
      </div>
    {:else}
      <form class="tag-form" on:submit|preventDefault={handleSave}>
        <div class="form-scroll">
          <label>
            <span>Title</span>
            <input type="text" bind:value={track.title} required />
          </label>

          <label class="relative">
            <span>Artist</span>
            <input type="text" bind:value={track.artist} on:input={(e) => fetchSuggestions('artist', e.target.value)} required />
            {#if suggestions.length > 0 && activeSuggestionField === 'artist'}
              <div class="suggestions">
                {#each suggestions as s}
                  <button type="button" on:click={() => selectSuggestion(s)}>{s}</button>
                {/each}
              </div>
            {/if}
          </label>

          <label class="relative">
            <span>Album</span>
            <input type="text" bind:value={track.album} on:input={(e) => fetchSuggestions('album', e.target.value)} required />
            {#if suggestions.length > 0 && activeSuggestionField === 'album'}
              <div class="suggestions">
                {#each suggestions as s}
                  <button type="button" on:click={() => selectSuggestion(s)}>{s}</button>
                {/each}
              </div>
            {/if}
          </label>

          <div class="row">
            <label>
              <span>Year</span>
              <input type="number" bind:value={track.year} />
            </label>
            <label class="relative">
              <span>Genre</span>
              <input type="text" bind:value={track.genre} on:input={(e) => fetchSuggestions('genre', e.target.value)} />
              {#if suggestions.length > 0 && activeSuggestionField === 'genre'}
                <div class="suggestions">
                  {#each suggestions as s}
                    <button type="button" on:click={() => selectSuggestion(s)}>{s}</button>
                  {/each}
                </div>
              {/if}
            </label>
          </div>
...

        <div class="row">
          <label>
            <span>Track #</span>
            <input type="number" bind:value={track.trackNum} placeholder="1" />
          </label>
          <label>
            <span>Disc #</span>
            <input type="number" bind:value={track.discNum} placeholder="1" />
          </label>
        </div>

        <label>
          <span>Composer</span>
          <input type="text" bind:value={track.composer} placeholder="Composer name..." />
        </label>

        <label>
          <span>Comment</span>
          <input type="text" bind:value={track.comment} placeholder="Your notes..." />
        </label>

        <label class="lyrics-label">
          <div class="label-header">
            <span>Lyrics</span>
            <button type="button" class="mini-btn" on:click={downloadLyrics}>☁️ Download</button>
          </div>
          <textarea bind:value={track.lyrics} placeholder="Lyrics here..."></textarea>
        </label>
      </div>

      <div class="actions">
        <button type="button" class="cancel-btn" on:click={closeModal}>Cancel</button>
        <button type="submit" class="save-btn" disabled={loading}>
          {loading ? 'Saving...' : saveSuccess ? 'Saved ✓' : 'Save Tags & Lyrics'}
        </button>
      </div>
    </form>
  </div>
</div>
{/if}

<style>
  .modal-backdrop {
    position: fixed;
    top: 0; left: 0; width: 100%; height: 100%;
    background: rgba(0,0,0,0.6);
    backdrop-filter: blur(8px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }

  .modal {
    background: #1e293b;
    padding: 2rem;
    border-radius: 32px;
    width: 95%;
    max-width: 500px;
    max-height: 90vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 25px 50px -12px rgba(0,0,0,0.5);
    border: 1px solid rgba(255,255,255,0.1);
  }

  .form-scroll {
    overflow-y: auto;
    padding-right: 0.5rem;
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    margin-bottom: 2rem;
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 2rem;
  }

  .title-with-btn {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .auto-btn {
    background: #ffcc00; /* Color llamativo para búsqueda */
    color: #1e293b;
    border: none;
    padding: 0.4rem 1rem;
    border-radius: 20px;
    font-size: 0.85rem;
    font-weight: 700;
    cursor: pointer;
    box-shadow: 0 4px 10px rgba(255, 204, 0, 0.3);
    width: fit-content;
  }

  .auto-btn:hover {
    transform: translateY(-2px);
    background: #ffd633;
  }

  .modal-header h2 { margin: 0; color: white; }

  .row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
  }

  .relative { position: relative; }

  .suggestions {
    position: absolute;
    top: 100%;
    left: 0;
    width: 100%;
    background: #1e293b;
    border: 1px solid rgba(255,255,255,0.1);
    border-radius: 12px;
    z-index: 10;
    margin-top: 4px;
    max-height: 200px;
    overflow-y: auto;
    box-shadow: 0 10px 25px rgba(0,0,0,0.5);
  }

  .suggestions button {
    display: block;
    width: 100%;
    text-align: left;
    padding: 0.8rem 1rem;
    background: transparent;
    border: none;
    color: white;
    cursor: pointer;
  }

  .suggestions button:hover { background: rgba(255, 51, 102, 0.2); }

  .results-picker {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    overflow-y: auto;
  }

  .results-list {
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
  }

  .result-item {
    background: rgba(255,255,255,0.05);
    border: 1px solid rgba(255,255,255,0.1);
    padding: 0.8rem;
    border-radius: 16px;
    display: flex;
    align-items: center;
    gap: 1rem;
    cursor: pointer;
    text-align: left;
    color: white;
  }

  .result-item:hover { border-color: #ff3366; background: rgba(255, 51, 102, 0.1); }

  .result-item img { width: 50px; height: 50px; border-radius: 8px; }

  .res-info { display: flex; flex-direction: column; }
  .res-title { font-weight: 700; font-size: 0.95rem; }
  .res-sub { font-size: 0.8rem; color: #94a3b8; }

  .tag-form label {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    color: #94a3b8;
  }

  .label-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .mini-btn {
    background: rgba(255, 51, 102, 0.1);
    color: #ff3366;
    border: none;
    padding: 0.3rem 0.6rem;
    border-radius: 6px;
    font-size: 0.8rem;
    font-weight: 600;
    cursor: pointer;
  }

  .tag-form input, .tag-form textarea {
    background: rgba(0,0,0,0.2);
    border: 1px solid rgba(255,255,255,0.1);
    color: white;
    padding: 0.8rem 1rem;
    border-radius: 12px;
    font-size: 1rem;
    outline: none;
  }

  .tag-form textarea {
    height: 150px;
    resize: vertical;
    font-family: inherit;
    line-height: 1.5;
  }

  .tag-form input:focus, .tag-form textarea:focus {
    border-color: #ff3366;
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 1rem;
  }

  .save-btn {
    background: #ff3366;
    color: white;
    padding: 0.8rem 2rem;
    border-radius: 12px;
    font-weight: 600;
    border: none;
    cursor: pointer;
  }

  .cancel-btn {
    background: transparent;
    border: none;
    color: #94a3b8;
    cursor: pointer;
  }
</style>
