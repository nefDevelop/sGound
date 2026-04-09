<script>
  import { createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';
  import { showToast } from './stores.js';
  
  let { 
    isOpen = $bindable(false), 
    track = $bindable({ id: '', title: '', artist: '', album: '' }),
    onclose = null
  } = $props();
  
  let loading = $state(false);
  let saveSuccess = $state(false);
  let searchResults = $state([]);
  let lyricSearchResults = $state([]); 
  let suggestions = $state([]);
  let activeSuggestionField = $state("");

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
          comment: track.comment,
          instrumental: track.instrumental
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
    if (onclose) onclose();
  }
</script>

{#if isOpen}
<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="modal-backdrop" onclick={closeModal}>
  <div class="modal glass" onclick={(e) => e.stopPropagation()}>
    <div class="modal-header">
      <h2>Edit Track Tags</h2>
      <button class="close-btn" onclick={closeModal}><Icon name="x" size={20} color="#94a3b8" /></button>
    </div>

    <div class="editor-content-wrap">
      <aside class="editor-sidebar">
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

        <div class="sidebar-actions">
          <button type="button" class="auto-btn mb" onclick={suggestMusicBrainz}>
            <Icon name="music" size={16} color="white" /> 
            <span>MusicBrainz Auto-Tag</span>
          </button>
          <button type="button" class="auto-btn" onclick={autoSearchTags}>
            <Icon name="zap" size={16} color="var(--primary)" /> 
            <span>Search Metadata</span>
          </button>
        </div>

        {#if searchResults.length > 0}
          <div class="results-picker">
            <div class="picker-header">
               <h3>Match results</h3>
               <p>Found {searchResults.length} matches</p>
            </div>
            <div class="results-list">
              {#each searchResults as res}
                <button class="result-item" onclick={() => applyMetadata(res)}>
                  <img src={res.artwork} alt="" />
                  <div class="res-info">
                    <span class="res-title">{res.title}</span>
                    <span class="res-sub">{res.artist} • {res.album} ({res.year})</span>
                  </div>
                </button>
              {/each}
            </div>
          </div>
        {/if}
      </aside>

      <div class="main-editor-area">
        <form class="tag-form" onsubmit={(e) => { e.preventDefault(); handleSave(); }}>
          <div class="form-scroll">
            <div class="form-grid">
              <label class="col-span-2">
                <span>Title</span>
                <input type="text" bind:value={track.title} required />
              </label>

              <label class="relative col-span-2">
                <span>Artist</span>
                <input type="text" bind:value={track.artist} oninput={(e) => fetchSuggestions('artist', e.currentTarget.value)} required />
                {#if suggestions.length > 0 && activeSuggestionField === 'artist'}
                  <div class="suggestions">
                    {#each suggestions as s}
                      <button type="button" onclick={() => selectSuggestion(s)}>{s}</button>
                    {/each}
                  </div>
                {/if}
              </label>

              <label class="relative col-span-2">
                <span>Album</span>
                <input type="text" bind:value={track.album} oninput={(e) => fetchSuggestions('album', e.currentTarget.value)} required />
                {#if suggestions.length > 0 && activeSuggestionField === 'album'}
                  <div class="suggestions">
                    {#each suggestions as s}
                      <button type="button" onclick={() => selectSuggestion(s)}>{s}</button>
                    {/each}
                  </div>
                {/if}
              </label>

              <label>
                <span>Year</span>
                <input type="number" bind:value={track.year} />
              </label>
              <label class="relative">
                <span>Genre</span>
                <input type="text" bind:value={track.genre} oninput={(e) => fetchSuggestions('genre', e.currentTarget.value)} />
                {#if suggestions.length > 0 && activeSuggestionField === 'genre'}
                  <div class="suggestions">
                    {#each suggestions as s}
                      <button type="button" onclick={() => selectSuggestion(s)}>{s}</button>
                    {/each}
                  </div>
                {/if}
              </label>

              <label>
                <span>Track #</span>
                <input type="number" bind:value={track.trackNum} placeholder="1" />
              </label>
              <label>
                <span>Disc #</span>
                <input type="number" bind:value={track.discNum} placeholder="1" />
              </label>

              <label class="col-span-2">
                <span>Composer</span>
                <input type="text" bind:value={track.composer} placeholder="Composer name..." />
              </label>

              <label class="checkbox-row col-span-2">
                <input type="checkbox" bind:checked={track.instrumental} />
                <span>Instrumental Track</span>
              </label>

              <label class="lyrics-label col-span-2">
                <div class="label-header">
                  <span>Lyrics</span>
                  <button type="button" class="mini-btn" onclick={searchLyrics}><Icon name="search" size={14} /> Search Online</button>
                </div>
                
                {#if lyricSearchResults.length > 0}
                  <div class="lyric-chips">
                    {#each lyricSearchResults as res}
                      <button type="button" class="lyric-chip {res.syncedLyrics ? 'sync' : ''}" onclick={() => selectLyric(res)}>
                         <Icon name={res.syncedLyrics ? "zap" : "fileText"} size={12} color={res.syncedLyrics ? "#1e293b" : "#94a3b8"} />
                         <span>{res.syncedLyrics ? 'SYNC' : 'Plain'}</span>
                      </button>
                    {/each}
                  </div>
                {/if}

                <textarea bind:value={track.lyrics} placeholder="Paste lyrics or search online..."></textarea>
              </label>

              <label class="col-span-2">
                <span>Comment</span>
                <input type="text" bind:value={track.comment} placeholder="Your notes..." />
              </label>
            </div>
          </div>

          <div class="actions">
            <button type="button" class="cancel-btn" onclick={closeModal}>Cancel</button>
            <button type="submit" class="save-btn" disabled={loading}>
              {loading ? 'Saving...' : saveSuccess ? 'Saved ✓' : 'Save Track Details'}
            </button>
          </div>
        </form>
      </div>
    </div>
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
    z-index: 5000;
  }

  .modal {
    background: var(--surface-container-highest);
    padding: 0;
    border-radius: 40px;
    width: 95%;
    max-width: 900px;
    height: 85vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 40px 120px rgba(0,0,0,0.6);
    border: 1px solid var(--border-color);
    overflow: hidden;
    box-sizing: border-box;
  }

  .editor-content-wrap {
    display: flex;
    flex: 1;
    overflow: hidden;
  }

  .editor-sidebar {
    width: 320px;
    background: rgba(0,0,0,0.2);
    border-right: 1px solid var(--border-color);
    padding: 2rem;
    display: flex;
    flex-direction: column;
    gap: 2rem;
    overflow-y: auto;
  }

  .main-editor-area {
    flex: 1;
    display: flex;
    flex-direction: column;
    padding: 2.5rem;
    overflow: hidden;
  }

  .form-scroll {
    overflow-y: auto;
    padding-right: 1rem;
    margin-bottom: 2rem;
    flex: 1;
    min-height: 0;
  }

  .form-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1.5rem;
  }

  .col-span-2 {
    grid-column: span 2;
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1.5rem 2rem;
    border-bottom: 1px solid var(--border-color);
  }

  .sidebar-actions {
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
  }

  .auto-btn {
    background: var(--surface-container-high);
    color: var(--text-main);
    border: 1px solid var(--border-color);
    padding: 0.8rem 1.2rem;
    border-radius: 16px;
    font-size: 0.85rem;
    font-weight: 700;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 0.8rem;
    transition: all 0.2s;
  }

  .auto-btn span { white-space: nowrap; }

  .auto-btn.mb {
    background: var(--primary);
    color: white;
    border: none;
    box-shadow: 0 4px 15px var(--primary-20);
  }

  .auto-btn:hover {
    transform: translateY(-2px);
    filter: brightness(1.1);
  }

  .modal-header h2 { margin: 0; color: white; font-size: 1.2rem; }

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

  .suggestions button:hover { background: var(--primary-10); color: var(--primary); }

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

  .checkbox-row {
    flex-direction: row !important;
    align-items: center;
    gap: 0.8rem !important;
    cursor: pointer;
    background: var(--surface-container);
    padding: 0.8rem 1.2rem;
    border-radius: 12px;
    border: 1px solid var(--border-color);
  }

  .checkbox-row input {
    margin: 0;
    width: 20px;
    height: 20px;
    flex-shrink: 0;
    accent-color: var(--primary);
    cursor: pointer;
  }

  .checkbox-row span {
    margin-top: 0 !important;
    color: var(--text-main) !important;
    font-weight: 600;
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
    background: var(--primary-10);
    color: var(--primary);
    border: none;
    padding: 0.3rem 0.6rem;
    border-radius: 8px;
    font-size: 0.8rem;
    font-weight: 700;
    cursor: pointer;
  }

  .tag-form input:not([type="checkbox"]), .tag-form textarea {
    background: var(--surface-container);
    border: 1px solid var(--border-color);
    color: var(--text-main);
    padding: 0.8rem 1.2rem;
    border-radius: 12px;
    font-size: 1rem;
    outline: none;
    width: 100%;
    box-sizing: border-box;
    transition: border-color 0.2s, background 0.2s;
  }

  .tag-form textarea {
    height: 150px;
    resize: vertical;
    font-family: inherit;
    line-height: 1.5;
  }

  .tag-form input:focus, .tag-form textarea:focus {
    border-color: var(--primary);
    background: var(--surface);
  }

  /* Hide number input spinners */
  input::-webkit-outer-spin-button,
  input::-webkit-inner-spin-button {
    -webkit-appearance: none;
    appearance: none;
    margin: 0;
  }
  input[type=number] {
    -moz-appearance: textfield;
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 1rem;
  }

  .save-btn {
    background: var(--primary);
    color: white;
    padding: 1rem 2.5rem;
    border-radius: 14px;
    font-weight: 700;
    border: none;
    cursor: pointer;
    box-shadow: 0 8px 20px var(--primary-10);
    transition: all 0.2s;
  }
  .save-btn:hover { transform: translateY(-2px); filter: brightness(1.1); }
  .save-btn:disabled { opacity: 0.5; filter: grayscale(1); }

  .cancel-btn {
    background: transparent;
    border: none;
    color: #94a3b8;
    cursor: pointer;
    font-weight: 600;
    padding: 0.8rem;
  }

  .cover-canvas {
      position: relative;
      width: 100%;
      aspect-ratio: 1;
      border-radius: 24px;
      overflow: hidden;
      flex-shrink: 0;
      box-shadow: 0 12px 40px rgba(0,0,0,0.4);
      margin-bottom: 1rem;
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

  .hero-info h3 { margin: 0; font-size: 1.2rem; color: white; font-weight: 900; }
  .hero-info p { margin: 0.3rem 0 0; font-size: 1rem; color: #94a3b8; font-weight: 600; }

  .picker-header { margin-bottom: 1rem; }
  .picker-header h3 { margin: 0; color: white; font-size: 1.2rem; }
  .picker-header p { margin: 0.3rem 0; font-size: 0.85rem; color: #94a3b8; }


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
