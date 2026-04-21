<script>
  import { createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';
  import { showToast } from './stores.js';
  import { fade } from 'svelte/transition';
  
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
  let albumSearchResults = $state([]);
  let showAlbumResults = $state(false);

  let isSingleAlbum = $derived(
    selectedTracks.length > 0 && 
    selectedTracks.every(t => t.album === selectedTracks[0].album && t.artist === selectedTracks[0].artist)
  );
  let albumInfo = $derived(isSingleAlbum ? {
    title: selectedTracks[0].album,
    artist: selectedTracks[0].artist,
    coverTrackId: selectedTracks[0].id
  } : null);

  // Datos globales que se aplicarán a todos
  let bulkData = $state({
    artist: '',
    albumArtist: '',
    album: '',
    genre: '',
    year: '',
    composer: '',
    trackTotal: ''
  });

  let dragOverIdx = $state(-1);
  let draggingIdx = $state(-1);
  let editingTrackId = $state(null);
  let editingField = $state(null);

  // Generar la tabla basada en slots
  let trackGrid = $derived.by(() => {
    const total = parseInt(bulkData.trackTotal) || 0;
    const maxTrackNum = selectedTracks.reduce((max, t) => Math.max(max, t.trackNum || 0), 0);
    const gridLen = Math.max(total, maxTrackNum, selectedTracks.length);
    
    const grid = Array(gridLen).fill(null);
    const unplaced = [];

    // Mapear tracks a sus posiciones actuales
    selectedTracks.forEach(t => {
      const pos = (t.trackNum || 0) - 1;
      if (pos >= 0 && pos < gridLen && !grid[pos]) {
        grid[pos] = t;
      } else {
        unplaced.push(t);
      }
    });

    // Colocar los tracks que no tienen posición o están en conflicto (sin mutar)
    unplaced.forEach(t => {
      const firstEmpty = grid.findIndex(v => v === null);
      if (firstEmpty !== -1) {
        grid[firstEmpty] = t;
      }
    });

    return grid;
  });

  function handleDragStart(idx, e, isSlot = false) {
    if (isSlot && !trackGrid[idx]) return; 
    draggingIdx = idx;
    e.dataTransfer.effectAllowed = 'move';
  }

  function handleDragOver(idx, e) {
    e.preventDefault();
    dragOverIdx = idx;
  }

  function handleDrop(idx, e) {
    e.preventDefault();
    if (draggingIdx === -1 || draggingIdx === idx) {
      draggingIdx = -1;
      dragOverIdx = -1;
      return;
    }

    // El origen es un track en draggingIdx, el destino es el slot idx
    const total = parseInt(bulkData.trackTotal) || selectedTracks.length;
    const sourceTrack = trackGrid[draggingIdx];
    if (!sourceTrack) return;

    // Actualizar trackNum de todos los tracks según su nueva posición en el grid imaginario
    const newTracks = [...selectedTracks];
    const targetTrack = trackGrid[idx];

    // Intercambiar trackNums
    if (targetTrack) {
      const oldSourceNum = sourceTrack.trackNum;
      sourceTrack.trackNum = idx + 1;
      targetTrack.trackNum = draggingIdx + 1;
    } else {
      sourceTrack.trackNum = idx + 1;
    }
    
    selectedTracks = [...newTracks];
    draggingIdx = -1;
    dragOverIdx = -1;
  }

  function startEdit(trackId, field) {
    editingTrackId = trackId;
    editingField = field;
  }

  function handleEditKeydown(e) {
    if (e.key === 'Enter' || e.key === 'Escape') {
      editingTrackId = null;
      editingField = null;
    }
  }

  function focusOnMount(node) {
    node.focus();
  }

  let initialized = $state(false);
  // Pre-poblar campos si son comunes a todos los tracks (solo al abrir)
  $effect(() => {
    if (isOpen && selectedTracks.length > 0 && !initialized) {
      const getCommon = (field) => {
        const firstVal = selectedTracks[0][field];
        if (selectedTracks.every(t => t[field] === firstVal)) return firstVal;
        return '';
      };
      
      bulkData = {
        artist: getCommon('artist'),
        albumArtist: getCommon('albumArtist'),
        album: getCommon('album'),
        genre: getCommon('genre'),
        year: getCommon('year') || '',
        composer: getCommon('composer'),
        trackTotal: getCommon('trackTotal') || ''
      };

      // Normalizar trackNums si faltan (sin disparar el effect infinitamente)
      let nextNum = 1;
      selectedTracks.forEach(t => {
        if (!t.trackNum) {
          // Buscar el primer número libre
          while (selectedTracks.some(other => other.trackNum === nextNum)) {
            nextNum++;
          }
          t.trackNum = nextNum;
        }
      });

      initialized = true;
    }
    if (!isOpen) {
      initialized = false;
    }
  });

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
      const parts = bulkData.genre.split(/[;/]/);
      parts[parts.length - 1] = " " + val;
      bulkData.genre = parts.join("; ").trim();
    } else {
      bulkData[activeSuggestionField] = val;
    }
    suggestions = [];
  }

  async function handleBulkSave() {
    if (selectedTracks.length === 0) return;
    loading = true;
    processingStatus = "Saving all tracks...";
    
    try {
      const promises = selectedTracks.map((track, index) => {
        const payload = { ...track };
        if (bulkData.artist) payload.artist = bulkData.artist;
        if (bulkData.albumArtist) payload.albumArtist = bulkData.albumArtist;
        if (bulkData.album) payload.album = bulkData.album;
        if (bulkData.genre) payload.genre = bulkData.genre;
        if (bulkData.year) payload.year = parseInt(bulkData.year);
        if (bulkData.composer) payload.composer = bulkData.composer;
        if (bulkData.trackTotal) payload.trackTotal = parseInt(bulkData.trackTotal);
        
        // El trackNum ya viene establecido por la posición en el grid/slots
        // payload.trackNum = track.trackNum; // No hace falta cambiarlo, ya viene en track

        const p1 = fetch(`/api/tracks/${track.id}/tags`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        }).then(async res => {
          const result = await res.json();
          if (!res.ok) {
            console.error(`[Bulk Error] ${track.title}:`, result.message);
            throw new Error(`Failed to update ${track.title}: ${result.message}`);
          }
          return result;
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

  async function searchAlbumMetadata() {
    const artist = bulkData.artist || (selectedTracks.length > 0 ? selectedTracks[0].artist : "");
    const album = bulkData.album || (selectedTracks.length > 0 ? selectedTracks[0].album : "");
    
    if (!artist || !album) {
      showToast("Artist and Album are required for search", "warning");
      return;
    }

    loading = true;
    processingStatus = "Searching album metadata...";
    try {
      const res = await fetch(`/api/metadata/search-album?artist=${encodeURIComponent(artist)}&album=${encodeURIComponent(album)}`);
      if (res.ok) {
        albumSearchResults = await res.json();
        showAlbumResults = true;
      }
    } catch (e) {
      showToast("Search failed", "error");
    } finally {
      loading = false;
      processingStatus = "";
    }
  }

  async function selectAlbumRelease(release) {
    loading = true;
    processingStatus = "Fetching tracklist details...";
    showAlbumResults = false;
    try {
      const res = await fetch(`/api/metadata/release-details?id=${release.id}`);
      if (res.ok) {
        const details = await res.json();
        bulkData.album = details.title;
        if (details.date) bulkData.year = details.date.split("-")[0];
        
        let totalTracks = 0;
        let mbTracks = [];
        details.media.forEach(m => {
          m.tracks.forEach(t => {
            mbTracks.push(t);
            totalTracks++;
          });
        });
        bulkData.trackTotal = totalTracks.toString();

        // Intentar mapear tracks locales a los de MusicBrainz por título (fuzzy)
        selectedTracks.forEach(local => {
          const match = mbTracks.find(mb => 
            local.title.toLowerCase().includes(mb.title.toLowerCase()) ||
            mb.title.toLowerCase().includes(local.title.toLowerCase())
          );
          if (match) {
            local.trackNum = parseInt(match.number);
            local.title = match.title; // Opcional: actualizar al título oficial
          }
        });
        
        showToast(`Metadata applied from ${details.title}`, "success");
      }
    } catch (e) {
      showToast("Failed to fetch release details", "error");
    } finally {
      loading = false;
      processingStatus = "";
    }
  }

  function sortTracksByTitle() {
    selectedTracks = [...selectedTracks].sort((a, b) => a.title.localeCompare(b.title));
    // Reasignar números de pista secuencialmente tras ordenar
    selectedTracks.forEach((t, i) => t.trackNum = i + 1);
    showToast("Tracks sorted by title", "info");
  }

  async function autoSearchAll() {
    loading = true;
    let count = 0;
    // Antes de buscar, si no tienen número, ordenarlos por título para tener una base
    if (selectedTracks.every(t => !t.trackNum || t.trackNum === 0)) {
        selectedTracks.sort((a, b) => a.title.localeCompare(b.title));
    }

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
        <div class="header-info-wrap">
          {#if albumInfo}
            <div class="header-cover">
              <img src="/api/cover/{albumInfo.coverTrackId}" alt="" />
            </div>
          {/if}
          <div class="header-info">
            <h2>{albumInfo ? 'Album Editor' : 'Bulk Edit Tags'}</h2>
            {#if albumInfo}
               <div class="album-meta">
                 <span class="album-title">{albumInfo.title}</span>
                 <span class="album-artist">{albumInfo.artist}</span>
               </div>
            {:else}
              <p class="subtitle">{selectedTracks.length} tracks selected</p>
            {/if}
          </div>
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
            <span>Album Artist</span>
            <input type="text" bind:value={bulkData.albumArtist} oninput={(e) => fetchSuggestions('artist', e.currentTarget.value)} />
            {#if suggestions.length > 0 && activeSuggestionField === 'albumArtist'}
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
          <label><span>Tracks (Total)</span><input type="number" bind:value={bulkData.trackTotal} placeholder="e.g. 12" /></label>
          
          <div class="bulk-actions">
            <button class="tool-btn highlight" onclick={searchAlbumMetadata} disabled={loading} title="Search full album metadata">
              <Icon name="search" size={14} /> Search Album
            </button>
            <button class="tool-btn" onclick={sortTracksByTitle} disabled={loading} title="Sort local tracks by title A-Z">
              <Icon name="shuffle" size={14} /> Sort A-Z
            </button>
            <button class="tool-btn" onclick={autoSearchAll} disabled={loading}><Icon name="zap" size={14} /> Auto-Tag</button>
            <button class="tool-btn" onclick={downloadAllLyrics} disabled={loading}><Icon name="cloud" size={14} /> Lyrics</button>
          </div>
        </div>
      </aside>

      <section class="tracks-list-area">
        {#if showAlbumResults}
          <div class="album-results-overlay" in:fade>
            <div class="results-modal glass">
              <div class="results-header">
                <h3>Select Release</h3>
                <button class="close-btn" onclick={() => showAlbumResults = false}><Icon name="x" size={18} /></button>
              </div>
              <div class="results-list">
                {#each albumSearchResults as rel}
                  <button class="result-item" onclick={() => selectAlbumRelease(rel)}>
                    <span class="rel-title">{rel.title}</span>
                    <span class="rel-date">{rel.date || 'Unknown year'}</span>
                  </button>
                {/each}
              </div>
            </div>
          </div>
        {/if}

        <div class="tracks-scroll">
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
              {#each trackGrid as track, i}
                <tr 
                  class="track-row"
                  class:empty-slot={!track}
                  draggable={!!track}
                  ondragstart={(e) => handleDragStart(i, e, true)}
                  ondragover={(e) => handleDragOver(i, e)}
                  ondrop={(e) => handleDrop(i, e)}
                  class:is-dragging={draggingIdx === i}
                  class:drag-over={dragOverIdx === i}
                >
                  <td class="drag-handle">
                    {#if track}
                      <Icon name="menu" size={14} color="var(--text-muted)" />
                    {/if}
                  </td>
                  <td class="track-num-cell">
                    <span class="slot-num" class:out-of-range={parseInt(bulkData.trackTotal) > 0 && (i + 1) > parseInt(bulkData.trackTotal)}>
                      {i + 1}
                    </span>
                  </td>
                  <td class="title-cell" ondblclick={() => track && startEdit(track.id, 'title')}>
                    {#if track}
                      {#if editingTrackId === track.id && editingField === 'title'}
                        <input 
                          type="text" 
                          bind:value={track.title} 
                          onkeydown={handleEditKeydown}
                          onblur={() => { editingTrackId = null; editingField = null; }}
                          use:focusOnMount
                          class="inline-edit"
                        />
                      {:else}
                        <span class="bold">{track.title}</span>
                      {/if}
                    {:else}
                      <span class="empty-hint">── Empty Slot ──</span>
                    {/if}
                  </td>
                  <td class="muted">{track ? track.album : ''}</td>
                  <td>
                    {#if track}
                      {#if track.lyrics}
                        <Icon name="checkCircle" size={14} color="#10b981" />
                      {:else}
                        <Icon name="xCircle" size={14} color="#f87171" />
                      {/if}
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
        
        {#if processingStatus}
          <div class="bottom-status" in:fade>
            <div class="spinner"></div>
            <span>{processingStatus}</span>
          </div>
        {/if}
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
  .header-info-wrap { display: flex; align-items: center; gap: 1.5rem; }
  .header-cover { width: 60px; height: 60px; border-radius: 12px; overflow: hidden; box-shadow: 0 8px 20px rgba(0,0,0,0.3); border: 1px solid var(--white-05); }
  .header-cover img { width: 100%; height: 100%; object-fit: cover; }
  
  .album-meta { display: flex; flex-direction: column; gap: 0.1rem; }
  .album-title { font-size: 1.1rem; font-weight: 800; color: white; letter-spacing: -0.5px; }
  .album-artist { font-size: 0.85rem; font-weight: 600; color: var(--primary); }

  .header-info h2 { margin: 0; font-size: 0.85rem; font-weight: 900; color: var(--text-muted); text-transform: uppercase; letter-spacing: 1.5px; margin-bottom: 2px; }
  .header-info .subtitle { margin: 0; font-size: 0.85rem; color: var(--text-muted); font-weight: 600; }

  .bottom-status {
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    background: var(--primary);
    color: white;
    padding: 0.8rem 2.5rem;
    display: flex;
    align-items: center;
    gap: 1rem;
    font-weight: 700;
    font-size: 0.9rem;
    z-index: 50;
    animation: slideUp 0.3s ease-out;
  }
  
  .spinner {
    width: 18px; height: 18px; border: 2px solid rgba(255,255,255,0.3); border-top-color: white;
    border-radius: 50%; animation: spin 1s linear infinite;
  }
  @keyframes spin { to { transform: rotate(360deg); } }
  @keyframes slideUp { from { transform: translateY(100%); } to { transform: translateY(0); } }

  .bulk-layout { display: grid; grid-template-columns: 340px 1fr; gap: 0; flex: 1; overflow: hidden; position: relative; }

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

  .bulk-actions { display: grid; grid-template-columns: 1fr 1fr; gap: 0.8rem; margin-top: 0.5rem; }

  .tool-btn {
    background: var(--surface-container-high); 
    color: var(--text-main);
    border: 1px solid var(--border-color); 
    padding: 0.8rem;
    border-radius: 16px; 
    font-size: 0.75rem; 
    font-weight: 700; 
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    transition: all 0.2s;
  }
  .tool-btn:hover { background: var(--surface-container-highest); border-color: var(--primary-30); color: var(--primary); }
  .tool-btn.highlight { border-color: var(--primary-50); background: var(--primary-10); color: var(--primary); }
  .tool-btn.highlight:hover { background: var(--primary); color: white; }

  .album-results-overlay {
    position: absolute;
    top: 0; left: 0; width: 100%; height: 100%;
    background: rgba(0,0,0,0.4);
    backdrop-filter: blur(4px);
    z-index: 100;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 2rem;
  }

  .results-modal {
    background: var(--surface-container-highest);
    border: 1px solid var(--border-color);
    border-radius: 24px;
    width: 100%;
    max-width: 500px;
    max-height: 80%;
    display: flex;
    flex-direction: column;
    box-shadow: 0 20px 60px rgba(0,0,0,0.5);
    overflow: hidden;
  }

  .results-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1.2rem 1.5rem;
    border-bottom: 1px solid var(--border-color);
  }
  .results-header h3 { margin: 0; font-size: 1rem; font-weight: 800; }

  .results-list {
    flex: 1;
    overflow-y: auto;
    padding: 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .result-item {
    background: var(--surface-container);
    border: 1px solid var(--border-color);
    padding: 1rem;
    border-radius: 12px;
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    text-align: left;
    cursor: pointer;
    transition: all 0.2s;
  }
  .result-item:hover {
    background: var(--primary-10);
    border-color: var(--primary);
  }
  .rel-title { font-weight: 700; color: white; }
  .rel-date { font-size: 0.8rem; color: var(--text-muted); }

  .tracks-list-area {
    position: relative;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  
  .tracks-scroll {
    flex: 1;
    overflow-y: auto;
    padding: 2.5rem;
  }

  .mini-tracks-table { width: 100%; border-collapse: separate; border-spacing: 0 4px; font-size: 0.9rem; }
  .mini-tracks-table th { text-align: left; color: var(--text-muted); padding: 0.8rem 1rem; font-weight: 800; text-transform: uppercase; font-size: 0.7rem; letter-spacing: 1px; }
  .mini-tracks-table td { padding: 0.8rem 1rem; border-top: 1px solid transparent; border-bottom: 1px solid transparent; color: var(--text-main); }
  
  .mini-tracks-table tr:hover td { background: var(--surface-container-high); }
  .mini-tracks-table td:first-child { border-top-left-radius: 12px; border-bottom-left-radius: 12px; }
  .mini-tracks-table td:last-child { border-top-right-radius: 12px; border-bottom-right-radius: 12px; }
  
  .mini-tracks-table .bold { font-weight: 700; color: white; }
  .mini-tracks-table .muted { color: var(--text-muted); opacity: 0.8; }

  .drag-handle { width: 30px; cursor: grab; opacity: 0.5; transition: opacity 0.2s; }
  .track-row:hover .drag-handle { opacity: 1; }
  
  .is-dragging { opacity: 0.4; background: var(--surface-container-highest) !important; }
  .drag-over { border-bottom: 2px solid var(--primary); }

  .track-num-cell { width: 40px; }
  .slot-num { font-weight: 900; color: var(--primary); font-size: 1rem; opacity: 0.8; }
  .slot-num.out-of-range { color: #f87171; }
  
  .empty-slot { opacity: 0.3; }
  .empty-hint { font-size: 0.8rem; letter-spacing: 1px; color: var(--text-muted); font-style: italic; }

  .title-cell { min-width: 250px; cursor: text; }
  .inline-edit {
    width: 100%;
    background: var(--surface-container-high);
    border: 1px solid var(--primary);
    border-radius: 6px;
    padding: 4px 8px;
    color: white;
    font-size: 0.9rem;
    font-weight: 700;
  }

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
