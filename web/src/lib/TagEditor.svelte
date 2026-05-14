<script>
  import { createEventDispatcher } from 'svelte';
  import { fade } from 'svelte/transition';
  import Icon from './Icon.svelte';
  import Artwork from './Artwork.svelte';
  import CoverSearcher from './CoverSearcher.svelte';
  import { showToast, coverRefreshTrigger } from './stores.js';
  
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
  let applyCoverToAlbum = $state(false);
  let isCoverSearcherOpen = $state(false);
  let fileInput;

  async function handleSave() {
    loading = true;
    try {
      const res = await fetch(`/api/tracks/${track.id}/tags`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          title: track.title,
          artist: track.artist,
          album: track.album,
          albumArtist: track.albumArtist,
          genre: track.genre,
          year: parseInt(track.year) || 0,
          trackNum: parseInt(track.trackNum) || 0,
          trackTotal: parseInt(track.trackTotal) || 0,
          discNum: parseInt(track.discNum) || 0,
          discTotal: parseInt(track.discTotal) || 0,
          composer: track.composer,
          comment: track.comment,
          instrumental: !!track.instrumental
        })
      });

      const result = await res.json();
      if (!res.ok) throw new Error(result.message || "Failed to update tags");

      if (track.lyrics) {
        await fetch(`/api/lyrics/${track.id}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ lyrics: track.lyrics })
        });
      }
      
      saveSuccess = true;
      coverRefreshTrigger.set(Date.now());
      setTimeout(() => {
        saveSuccess = false;
        closeModal();
      }, 1500);
      
    } catch (e) {
      showToast(e.message, "error");
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
      lyricSearchResults = results || [];
      if (lyricSearchResults.length === 0) showToast("No lyrics found", "info");
    } catch (e) {
      showToast("Error searching lyrics", "error");
    } finally {
      loading = false;
    }
  }

  function selectLyric(lyricData) {
    track.lyrics = lyricData.syncedLyrics || lyricData.plainLyrics;
    lyricSearchResults = [];
  }

  async function fetchSuggestions(field, value) {
    let query = value;
    if (field === 'genre') {
      const parts = value.split(/[;/]/);
      query = parts[parts.length - 1].trim();
    }
    if (query.length < 1) {
      suggestions = [];
      return;
    }
    try {
      const res = await fetch(`/api/autocomplete?field=${field}&q=${encodeURIComponent(query)}`);
      suggestions = await res.json();
      activeSuggestionField = field;
    } catch (e) { console.error(e); }
  }

  function selectSuggestion(val) {
    if (activeSuggestionField === 'genre') {
      const parts = track.genre.split(/[;/]/);
      parts[parts.length - 1] = " " + val;
      track.genre = parts.join("; ").trim();
    } else {
      track[activeSuggestionField] = val;
    }
    suggestions = [];
  }

  async function autoSearchTags() {
    loading = true;
    try {
      const res = await fetch(`/api/metadata/search?artist=${encodeURIComponent(track.artist)}&title=${encodeURIComponent(track.title)}`);
      if (!res.ok) throw new Error("Metadata not found");
      searchResults = await res.json();
      if (searchResults.length === 1) applyMetadata(searchResults[0]);
      else if (searchResults.length === 0) showToast("No metadata found", "info");
    } catch (e) {
      showToast("Auto search failed", "error");
    } finally {
      loading = false;
    }
  }

  async function suggestMusicBrainz() {
    loading = true;
    try {
      const res = await fetch(`/api/metadata/suggest?id=${encodeURIComponent(track.id)}`);
      if (!res.ok) throw new Error("MusicBrainz suggestion failed");
      const results = await res.json();
      if (results && results.length > 0) {
        const bestMatch = results.find(s => s.title.toLowerCase().includes(track.title.toLowerCase())) || results[0];
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
    track.albumArtist = data.albumArtist || data.artist || track.albumArtist;
    track.title = data.title || track.title;
    track.album = data.album || track.album;
    track.year = data.year || track.year;
    track.genre = data.genre || track.genre;
    track.trackNum = data.trackNum || track.trackNum;
    searchResults = [];
  }

  async function uploadCoverFile(file) {
    if (!file) return;
    
    loading = true;
    const formData = new FormData();
    formData.append('cover', file);
    
    try {
      const res = await fetch(`/api/tracks/${track.id}/cover?album=${applyCoverToAlbum}`, {
        method: 'POST',
        body: formData
      });
      if (res.ok) {
        showToast("Cover updated from clipboard!", "success");
        coverRefreshTrigger.set(Date.now());
      } else {
        const error = await res.text();
        showToast("Update failed: " + error, "error");
      }
    } catch (e) {
        showToast("Error updating cover", "error");
    } finally {
        loading = false;
    }
  }

  function closeModal() {
    isOpen = false;
    if (onclose) onclose();
  }

  $effect(() => {
    if (isOpen && track.id) {
      fetch(`/api/lyrics/${track.id}`)
        .then(res => res.json())
        .then(data => { if (data.lyrics) track.lyrics = data.lyrics; })
        .catch(() => {});
    }
  });
</script>

{#if isOpen}
<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="modal-backdrop" onclick={closeModal} onpaste={(e) => {
  const items = e.clipboardData.items;
  for (let item of items) {
    if (item.type.indexOf('image') !== -1) {
      uploadCoverFile(item.getAsFile());
      break;
    }
  }
}}>
    <div class="modal glass" onclick={(e) => e.stopPropagation()}>
      <div class="modal-header">
        <div class="header-info-wrap">
          <Artwork 
            src="/api/cover/{track.id}" 
            size="56px" 
            borderRadius="12px"
            showSearchOverlay={true}
            onclick={() => (isCoverSearcherOpen = true)}
          />
          <div class="header-info">
            <h2>Edit Track Tags</h2>
            <p class="subtitle">{track.title || 'Untitled'}</p>
          </div>
        </div>
        <button class="close-btn" onclick={closeModal}><Icon name="x" size={20} color="#94a3b8" /></button>
      </div>

    <div class="bulk-layout">
      <aside class="bulk-sidebar">
        <h3>Magic Tools</h3>
        <p class="hint">Auto-fill metadata from internet</p>
        
        <div class="sidebar-actions">
          <button type="button" class="tool-btn highlight" onclick={suggestMusicBrainz}>
            <Icon name="music" size={16} /> <span>MusicBrainz</span>
          </button>
          <button type="button" class="tool-btn" onclick={autoSearchTags} disabled={loading}>
            <Icon name="zap" size={16} /> <span>Auto-Search</span>
          </button>
          <button type="button" class="tool-btn" onclick={searchLyrics} disabled={loading}>
            <Icon name="cloud" size={16} /> <span>Search Lyrics</span>
          </button>
        </div>

        <div class="cover-options">
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={applyCoverToAlbum} />
            <span>Apply cover to whole album</span>
          </label>
        </div>

        {#if searchResults.length > 0}
          <div class="results-picker" in:fade>
            <div class="picker-header"><h3>Results</h3></div>
            <div class="results-list">
              {#each searchResults as res}
                <button class="result-item" onclick={() => applyMetadata(res)}>
                  <span class="res-title">{res.title}</span>
                  <span class="res-sub">{res.artist} • {res.album}</span>
                </button>
              {/each}
            </div>
          </div>
        {/if}

        {#if lyricSearchResults.length > 0}
          <div class="results-picker" in:fade>
            <div class="picker-header"><h3>Lyrics Found</h3></div>
            <div class="results-list">
              {#each lyricSearchResults as res}
                <button class="result-item" onclick={() => selectLyric(res)}>
                  <span class="res-title">{res.syncedLyrics ? '⚡ SYNCED' : 'Plain Text'}</span>
                  <span class="res-sub">{res.artist} - {res.title}</span>
                </button>
              {/each}
            </div>
          </div>
        {/if}
      </aside>

      <div class="tracks-list-area">
        <form class="tag-form" onsubmit={(e) => { e.preventDefault(); handleSave(); }}>
          <div class="tracks-scroll">
            <div class="form-grid">
              <label class="col-span-2">
                <span>Title</span>
                <input type="text" bind:value={track.title} required />
              </label>

              <label class="relative">
                <span>Artist</span>
                <input type="text" bind:value={track.artist} oninput={(e) => fetchSuggestions('artist', e.currentTarget.value)} />
                {#if suggestions.length > 0 && activeSuggestionField === 'artist'}
                  <div class="suggestions">
                    {#each suggestions as s}
                      <button type="button" onclick={() => selectSuggestion(s)}>{s}</button>
                    {/each}
                  </div>
                {/if}
              </label>

              <label class="relative">
                <span>Album Artist</span>
                <input type="text" bind:value={track.albumArtist} oninput={(e) => fetchSuggestions('artist', e.currentTarget.value)} />
                {#if suggestions.length > 0 && activeSuggestionField === 'albumArtist'}
                  <div class="suggestions">
                    {#each suggestions as s}
                      <button type="button" onclick={() => selectSuggestion(s)}>{s}</button>
                    {/each}
                  </div>
                {/if}
              </label>

              <label class="col-span-2 relative">
                <span>Album</span>
                <input type="text" bind:value={track.album} oninput={(e) => fetchSuggestions('album', e.currentTarget.value)} />
                {#if suggestions.length > 0 && activeSuggestionField === 'album'}
                  <div class="suggestions">
                    {#each suggestions as s}
                      <button type="button" onclick={() => selectSuggestion(s)}>{s}</button>
                    {/each}
                  </div>
                {/if}
              </label>

              <label><span>Year</span><input type="number" bind:value={track.year} /></label>
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

              <label class="col-span-2"><span>Composer</span><input type="text" bind:value={track.composer} /></label>

              <div class="row-group">
                <label><span>Track #</span><input type="number" bind:value={track.trackNum} /></label>
                <label><span>of</span><input type="number" bind:value={track.trackTotal} /></label>
              </div>

              <div class="row-group">
                <label><span>Disc #</span><input type="number" bind:value={track.discNum} /></label>
                <label><span>of</span><input type="number" bind:value={track.discTotal} /></label>
              </div>

              <label class="col-span-2">
                <span>Comment</span>
                <input type="text" bind:value={track.comment} placeholder="Optional comment..." />
              </label>

              <div class="col-span-2 toggle-area">
                <label class="checkbox-label">
                  <input type="checkbox" bind:checked={track.instrumental} />
                  <span>Mark as Instrumental</span>
                </label>
              </div>

              <label class="col-span-2 lyrics-label">
                <span>Lyrics</span>
                <textarea bind:value={track.lyrics} placeholder="Paste lyrics here..."></textarea>
              </label>
            </div>
          </div>

          <div class="footer-actions">
            <button type="button" class="cancel-btn" onclick={closeModal}>Cancel</button>
            <button type="submit" class="save-btn" disabled={loading}>
              {loading ? 'Saving...' : saveSuccess ? 'Saved ✓' : 'Apply Changes'}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</div>

<CoverSearcher 
  bind:isOpen={isCoverSearcherOpen}
  trackId={track.id}
  albumId={track.albumId || track.album_id} 
  initialArtist={track.artist}
  initialAlbum={track.album}
/>
{/if}

<style>
  /* ... (previous styles) ... */
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
    max-width: 1000px;
    max-height: 90vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 40px 120px rgba(0,0,0,0.6);
    border: 1px solid var(--border-color);
    overflow: hidden;
  }

  .modal-header { 
    display: flex; 
    justify-content: space-between; 
    align-items: center;
    padding: 1.5rem 2.5rem;
    border-bottom: 1px solid var(--border-color);
    flex-shrink: 0;
  }
  .header-info-wrap { display: flex; align-items: center; gap: 1.5rem; }
  .header-info h2 { margin: 0; font-size: 0.85rem; font-weight: 900; color: var(--text-muted); text-transform: uppercase; letter-spacing: 1.5px; }
  .header-info .subtitle { margin: 0; font-size: 1.1rem; color: white; font-weight: 800; }

  .bulk-layout { 
    display: grid; 
    grid-template-columns: 280px 1fr; 
    gap: 0; 
    flex: 1; 
    min-height: 0;
    overflow: hidden; 
  }
  
  @media (max-width: 768px) {
    .bulk-layout { grid-template-columns: 1fr; overflow-y: auto; display: block; }
    .modal { border-radius: 24px; height: 95vh; }
    .modal-header { padding: 1rem 1.5rem; }
    .footer-actions { padding: 1rem 1.5rem; flex-direction: column-reverse; }
    .save-btn { width: 100%; }
    .bulk-sidebar { border-right: none; border-bottom: 1px solid var(--border-color); overflow: visible; padding: 1.5rem; }
    .tracks-scroll { overflow: visible; padding: 1.5rem; }
  }

  .bulk-sidebar { 
    display: flex; flex-direction: column; gap: 1.5rem; padding: 2.5rem;
    background: rgba(0,0,0,0.15); border-right: 1px solid var(--border-color);
    overflow-y: auto;
  }
  .bulk-sidebar h3 { margin: 0; font-size: 1rem; color: white; font-weight: 800; }
  .hint { font-size: 0.8rem; color: var(--text-muted); margin-top: -1rem; }

  .sidebar-actions { display: flex; flex-direction: column; gap: 0.8rem; }
  .tool-btn {
    background: var(--surface-container-high); color: var(--text-main);
    border: 1px solid var(--border-color); padding: 0.8rem; border-radius: 14px;
    font-size: 0.8rem; font-weight: 700; cursor: pointer;
    display: flex; align-items: center; gap: 0.8rem; transition: all 0.2s;
  }
  .tool-btn:hover { background: var(--surface-container-highest); border-color: var(--primary-30); }
  .tool-btn.highlight { background: var(--primary); color: white; border: none; }

  .cover-options { margin-top: 1rem; padding: 1rem; background: rgba(0,0,0,0.1); border-radius: 14px; border: 1px solid var(--border-color); }
  .checkbox-label { display: flex; align-items: center; gap: 0.8rem; cursor: pointer; font-size: 0.8rem; font-weight: 700; color: var(--text-muted); }
  .checkbox-label input { width: auto; margin: 0; cursor: pointer; }

  .results-picker { margin-top: 1rem; border-top: 1px solid var(--border-color); padding-top: 1rem; }
  .results-list { display: flex; flex-direction: column; gap: 0.5rem; margin-top: 1rem; }
  .result-item {
    background: var(--surface-container); border: 1px solid var(--border-color);
    padding: 0.8rem; border-radius: 10px; display: flex; flex-direction: column;
    text-align: left; cursor: pointer; transition: all 0.2s;
  }
  .result-item:hover { border-color: var(--primary); background: var(--primary-10); }
  .res-title { font-weight: 700; color: white; font-size: 0.85rem; }
  .res-sub { font-size: 0.75rem; color: var(--text-muted); }

  .tracks-list-area { flex: 1; display: flex; flex-direction: column; min-height: 0; overflow: hidden; }
  .tracks-scroll { flex: 1; overflow-y: auto; padding: 2.5rem; }

  @media (max-width: 480px) {
    .tracks-scroll { padding: 1.5rem; }
    .form-grid { grid-template-columns: 1fr; }
  }

  .form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 1.5rem; }
  .col-span-2 { grid-column: span 2; }
  .row-group { display: flex; align-items: flex-end; gap: 1rem; }
  .row-group label { flex: 1; }
  .row-group span { font-size: 0.7rem; opacity: 0.7; margin-bottom: 2px; }

  .toggle-area {
    background: rgba(0,0,0,0.1);
    padding: 1rem;
    border-radius: 14px;
    border: 1px solid var(--border-color);
  }

  .tag-form {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    height: 100%;
  }

  .tag-form label { 
    display: flex; flex-direction: column; gap: 0.5rem; 
    color: var(--text-muted); font-size: 0.85rem; font-weight: 700;
    min-width: 0; 
  }
  .tag-form input, .tag-form textarea {
    background: var(--surface-container); border: 1px solid var(--border-color);
    color: white; padding: 0.8rem 1.2rem; border-radius: 14px; outline: none; transition: all 0.2s;
    width: 100%;
    box-sizing: border-box;
  }
  .tag-form input:focus, .tag-form textarea:focus { border-color: var(--primary); background: var(--surface); }
  .tag-form textarea { height: 120px; font-family: inherit; }

  .tracks-scroll {
    flex: 1;
    overflow-y: auto;
    padding: 2.5rem;
    min-height: 0;
  }

  .relative { position: relative; }
  .suggestions {
    position: absolute; top: 100%; left: 0; width: 100%;
    background: var(--surface-container-highest); border: 1px solid var(--border-color);
    border-radius: 12px; z-index: 10; margin-top: 4px; max-height: 150px; overflow-y: auto;
  }
  .suggestions button {
    display: block; width: 100%; text-align: left; padding: 0.8rem 1rem;
    background: transparent; border: none; color: var(--text-main); cursor: pointer;
  }
  .suggestions button:hover { background: var(--primary-10); color: var(--primary); }

  .footer-actions {
    display: flex; justify-content: flex-end; gap: 1.5rem;
    padding: 1.5rem 2.5rem; border-top: 1px solid var(--border-color); 
    background: rgba(0,0,0,0.15);
    flex-shrink: 0;
  }
  .save-btn {
    background: var(--primary); color: white; padding: 1rem 3rem;
    border-radius: 100px; font-weight: 800; border: none; cursor: pointer;
    box-shadow: 0 10px 30px var(--primary-20); transition: all 0.2s;
  }
  .save-btn:hover { transform: translateY(-3px); }
  .cancel-btn { background: transparent; border: none; color: #64748b; font-weight: 600; cursor: pointer; padding: 0.8rem; }

  input::-webkit-outer-spin-button, input::-webkit-inner-spin-button { -webkit-appearance: none; appearance: none; margin: 0; }
  input[type=number] { -moz-appearance: textfield; appearance: textfield; }
</style>
