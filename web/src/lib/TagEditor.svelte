<script>
  import { createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';
  import { showToast } from './stores.js';
  
  export let isOpen = false;
  export let track = { id: '', title: '', artist: '', album: '' };
  
  const dispatch = createEventDispatcher();
  let loading = false;
  let saveSuccess = false;
  let searchResults = [];
  let lyricSearchResults = []; 
  let suggestions = [];
  let activeSuggestionField = "";

  async function handleSave() {
    loading = true;
    try {
      // 1. Guardar Tags completos
      const res = await fetch(`/api/tracks/tags?id=${encodeURIComponent(track.id)}`, {
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
      showToast("Error saving tags: " + e.message, "error");
    } finally {
      loading = false;
    }
  }

  async function searchLyrics() {
    loading = true;
    try {
      const res = await fetch(`/api/lyrics/search?artist=${encodeURIComponent(track.artist)}&title=${encodeURIComponent(track.title)}`);
      if (!res.ok) throw new Error("Search failed");
      const results = await res.json();
      
      if (results && results.length > 0) {
        lyricSearchResults = results;
      } else {
        showToast("No lyrics found for this track.", "info");
      }
    } catch (e) {
      showToast("Error searching lyrics: " + e.message, "error");
    } finally {
      loading = false;
    }
  }

  function selectLyric(lyricData) {
    // Preferir sincronizada si existe
    track.lyrics = lyricData.syncedLyrics || lyricData.plainLyrics;
    lyricSearchResults = [];
  }

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
      showToast("Auto search failed: " + e.message, "error");
    }
  }

  async function suggestMusicBrainz() {
    loading = true;
    try {
      const res = await fetch(`/api/metadata/suggest?id=${encodeURIComponent(track.id)}`);
      if (!res.ok) throw new Error("MusicBrainz suggestion failed");
      const suggestions = await res.json();
      if (suggestions && suggestions.length > 0) {
        // Encontrar la mejor coincidencia por título
        const bestMatch = suggestions.find(s => 
          s.title.toLowerCase().includes(track.title.toLowerCase()) || 
          track.title.toLowerCase().includes(s.title.toLowerCase())
        ) || suggestions[0];
        
        applyMetadata(bestMatch);
        showToast("Tags suggested from MusicBrainz", "success");
      } else {
        showToast("No matches in MusicBrainz", "info");
      }
    } catch (e) {
      showToast("MB Suggestion failed", "error");
    } finally {
      loading = false;
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

  function closeModal() {
    isOpen = false;
    dispatch('close');
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
        <div class="header-actions">
          <button type="button" class="auto-btn mb" on:click={suggestMusicBrainz} title="Auto-tag with MusicBrainz">
            <Icon name="music" size={16} color="white" /> Auto-Tag MB
          </button>
          <button type="button" class="auto-btn" on:click={autoSearchTags} title="Search Metadata">
            <Icon name="zap" size={16} color="#1e293b" /> Search
          </button>
        </div>
      </div>
      <button class="close-btn" on:click={closeModal}><Icon name="x" size={20} color="#94a3b8" /></button>
    </div>

    {#if searchResults.length > 0}
      <div class="results-picker">
        <div class="picker-header">
           <h3>Select correct metadata</h3>
           <p>Found {searchResults.length} matches</p>
        </div>
        <div class="results-list">
          {#each searchResults as res}
            <button class="result-item" on:click={() => applyMetadata(res)}>
              <img src={res.artwork} alt="" />
              <div class="res-info">
                <span class="res-title">{res.title}</span>
                <span class="res-sub">{res.artist} • {res.album} ({res.year})</span>
              </div>
            </button>
          {/each}
        </div>
        <button class="cancel-btn block-btn" on:click={() => searchResults = []}>Back to editor</button>
      </div>
    {:else}
      <form class="tag-form" on:submit|preventDefault={handleSave}>
        <div class="form-scroll">
          <div class="editor-hero">
            <div class="cover-canvas">
                <img src="/api/cover/{track.id}?t={Date.now()}" alt="Cover" />
                <div class="cover-overlay">
                    <Icon name="image" size={24} color="white" />
                </div>
            </div>
            <div class="hero-info">
                <h3>{track.title || 'Untitled'}</h3>
                <p>{track.artist || 'Unknown Artist'}</p>
            </div>
          </div>

          <label>
            <span>Title</span>
            <input type="text" bind:value={track.title} required />
          </label>

          <label class="relative">
            <span>Artist</span>
            <input type="text" bind:value={track.artist} on:input={(e) => fetchSuggestions('artist', e.currentTarget.value)} required />
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
            <input type="text" bind:value={track.album} on:input={(e) => fetchSuggestions('album', e.currentTarget.value)} required />
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
              <input type="text" bind:value={track.genre} on:input={(e) => fetchSuggestions('genre', e.currentTarget.value)} />
              {#if suggestions.length > 0 && activeSuggestionField === 'genre'}
                <div class="suggestions">
                  {#each suggestions as s}
                    <button type="button" on:click={() => selectSuggestion(s)}>{s}</button>
                  {/each}
                </div>
              {/if}
            </label>
          </div>


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
            <button type="button" class="mini-btn" on:click={searchLyrics}><Icon name="search" size={14} /> Search Online</button>
          </div>
          
          {#if lyricSearchResults.length > 0}
            <div class="lyric-chips">
              {#each lyricSearchResults as res}
                <button type="button" class="lyric-chip {res.syncedLyrics ? 'sync' : ''}" on:click={() => selectLyric(res)}>
                   <Icon name={res.syncedLyrics ? "zap" : "fileText"} size={12} color={res.syncedLyrics ? "#1e293b" : "#94a3b8"} />
                   <span>{res.syncedLyrics ? 'SYNC' : 'Plain'}</span>
                </button>
              {/each}
            </div>
          {/if}

          <textarea bind:value={track.lyrics} placeholder="Paste lyrics or search online..."></textarea>
        </label>
      </div>

      <div class="actions">
        <button type="button" class="cancel-btn" on:click={closeModal}>Cancel</button>
        <button type="submit" class="save-btn" disabled={loading}>
          {loading ? 'Saving...' : saveSuccess ? 'Saved ✓' : 'Save Tags & Lyrics'}
        </button>
      </div>
      </form>
    {/if}
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
    overflow: hidden;
    box-sizing: border-box;
  }

  .form-scroll {
    overflow-y: auto;
    padding-right: 0.5rem;
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    margin-bottom: 2rem;
    flex: 1;
    min-height: 0;
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

  .header-actions {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .auto-btn {
    background: #ffcc00;
    color: #1e293b;
    border: none;
    padding: 0.6rem 1.2rem;
    border-radius: 20px;
    font-size: 0.8rem;
    font-weight: 800;
    cursor: pointer;
    box-shadow: 0 4px 15px rgba(255, 204, 0, 0.2);
    width: fit-content;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    transition: all 0.2s;
  }

  .auto-btn.mb {
    background: var(--primary);
    color: white;
    box-shadow: 0 4px 15px var(--primary-10);
  }

  .auto-btn:hover {
    transform: translateY(-2px);
    filter: brightness(1.1);
  }

  .modal-header h2 { margin: 0; color: white; }

  .row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
    width: 100%;
  }

  @media (max-width: 400px) {
    .row {
      grid-template-columns: 1fr;
    }
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

  .tag-form {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
  }

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
    width: 100%;
    box-sizing: border-box;
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
    font-weight: 600;
    padding: 0.8rem;
  }

  .block-btn {
    width: 100%;
    background: rgba(255,255,255,0.05);
    margin-top: 1rem;
    border-radius: 12px;
  }

  .editor-hero {
     display: flex;
     gap: 1.5rem;
     align-items: center;
     background: rgba(255,255,255,0.03);
     padding: 1rem;
     border-radius: 20px;
     border: 1px solid rgba(255,255,255,0.05);
  }

  .cover-canvas {
      position: relative;
      width: 80px;
      height: 80px;
      border-radius: 12px;
      overflow: hidden;
      flex-shrink: 0;
      box-shadow: 0 8px 20px rgba(0,0,0,0.3);
  }

  .cover-canvas img {
      width: 100%;
      height: 100%;
      object-fit: cover;
  }

  .cover-overlay {
      position: absolute;
      top: 0; left: 0; width: 100%; height: 100%;
      background: rgba(0,0,0,0.4);
      display: flex;
      align-items: center;
      justify-content: center;
      opacity: 0;
      transition: opacity 0.2s;
  }

  .cover-canvas:hover .cover-overlay { opacity: 1; }

  .hero-info h3 { margin: 0; font-size: 1.1rem; color: white; }
  .hero-info p { margin: 0.2rem 0 0; font-size: 0.9rem; color: #94a3b8; }

  .picker-header { margin-bottom: 1rem; }
  .picker-header h3 { margin: 0; color: white; font-size: 1.2rem; }
  .picker-header p { margin: 0.3rem 0; font-size: 0.85rem; color: #94a3b8; }

  .lyric-icon-wrap {
      width: 40px;
      height: 40px;
      background: rgba(255,255,255,0.05);
      border-radius: 10px;
      display: flex;
      align-items: center;
      justify-content: center;
  }

  .lyric-chips {
      display: flex;
      gap: 0.5rem;
      overflow-x: auto;
      padding-bottom: 0.5rem;
      scrollbar-width: none;
  }
  .lyric-chips::-webkit-scrollbar { display: none; }

  .lyric-chip {
      display: flex;
      align-items: center;
      gap: 0.4rem;
      padding: 0.4rem 0.8rem;
      border-radius: 20px;
      background: rgba(255,255,255,0.05);
      border: 1px solid rgba(255,255,255,0.1);
      color: #94a3b8;
      font-size: 0.75rem;
      font-weight: 600;
      white-space: nowrap;
      cursor: pointer;
      transition: all 0.2s;
  }

  .lyric-chip:hover {
      background: rgba(255,255,255,0.1);
      border-color: #ff3366;
  }

  .lyric-chip.sync {
      background: #ffcc00;
      color: #1e293b;
      border: none;
  }
</style>
