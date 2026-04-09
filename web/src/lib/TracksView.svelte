<script>
  import Icon from './Icon.svelte';
  import ContextMenu from './ContextMenu.svelte';
  import StarRating from './StarRating.svelte';
  import { get } from 'svelte/store';
  
  let { 
    tracks = $bindable([]), 
    currentPlaying = null, 
    onplay, 
    onedit, 
    onselectionChange, 
    onqueue, 
    onhide, 
    onbulkReject,
    ondeletePhysical,
    isInbox = false
  } = $props();
  
  let selectedIds = $state(new Set());
  let sortConfig = $state({ field: 'title', direction: 'asc' });

  // Inline editing state
  let editingCell = $state({ trackId: null, field: null });
  let editingValue = $state('');
  let suggestions = $state([]);
  let suggestionIdx = $state(-1);

  let contextMenu = $state({
    visible: false,
    x: 0,
    y: 0,
    track: null,
    items: []
  });

  function showContextMenu(track, e) {
    e.preventDefault();
    contextMenu = {
      visible: true,
      x: e.clientX,
      y: e.clientY,
      track,
      items: [
        { label: 'Play', icon: 'play', action: () => onplay?.(track) },
        { label: 'Add to queue', icon: 'plus', action: () => onqueue?.(track) },
        { divider: true },
        { label: 'Edit tags', icon: 'edit', action: () => onedit?.(track) },
        ...(!isInbox ? [{ label: 'Organize file', icon: 'folder', action: () => handleOrganize(track.id) }] : []),
        { divider: true },
        { label: 'Copy title', icon: 'fileText', action: () => navigator.clipboard.writeText(track.title) },
        { divider: true },
        { label: 'Ocultar de la biblioteca', icon: 'eyeOff', action: () => onhide?.(track) },
        { label: 'Borrar físicamente (Disco)', icon: 'trash', color: 'danger', action: () => ondeletePhysical?.(track) }
      ]
    };
  }

  function handleRowClick(track, e) {
    // Si estamos editando o pulsamos el checkbox/rating, no seleccionar la fila
    if (editingCell.trackId || e.target.closest('input[type="checkbox"]') || e.target.closest('.col-rating')) return;
    
    toggleSelect(track.id);
  }

  function toggleSelect(id) {
    if (selectedIds.has(id)) {
      selectedIds.delete(id);
    } else {
      selectedIds.add(id);
    }
    selectedIds = new Set(selectedIds); 
    onselectionChange?.(Array.from(selectedIds).map(sid => tracks.find(t => t.id === sid)));
  }

  function toggleAll() {
    if (selectedIds.size === tracks.length) {
      selectedIds.clear();
    } else {
      tracks.forEach(t => selectedIds.add(t.id));
    }
    selectedIds = new Set(selectedIds);
    onselectionChange?.(Array.from(selectedIds).map(sid => tracks.find(t => t.id === sid)));
  }

  function toggleSort(field) {
    if (sortConfig.field === field) {
      sortConfig.direction = sortConfig.direction === 'asc' ? 'desc' : 'asc';
    } else {
      sortConfig.field = field;
      sortConfig.direction = 'asc';
    }

    tracks = [...tracks].sort((a, b) => {
      let valA = (a[field] || '').toString().toLowerCase();
      let valB = (b[field] || '').toString().toLowerCase();
      if (typeof a[field] === 'number') {
         valA = a[field];
         valB = b[field];
      }
      if (valA < valB) return sortConfig.direction === 'asc' ? -1 : 1;
      if (valA > valB) return sortConfig.direction === 'asc' ? 1 : -1;
      return 0;
    });
  }

  // --- Inline Editing Logic ---

  function startEdit(track, field, e) {
    if (isInbox) return;
    e.stopPropagation();
    editingCell = { trackId: track.id, field };
    editingValue = track[field] || '';
    suggestions = [];
    suggestionIdx = -1;
  }

  async function handleValueChange(e) {
    editingValue = e.target.value;
    if (['artist', 'album', 'genre'].includes(editingCell.field) && editingValue.length > 1) {
      const res = await fetch(`/api/autocomplete?field=${editingCell.field}&q=${encodeURIComponent(editingValue)}`);
      if (res.ok) suggestions = await res.json();
    } else {
      suggestions = [];
    }
  }

  async function saveEdit() {
    if (!editingCell.trackId) return;
    
    const track = tracks.find(t => t.id === editingCell.trackId);
    if (track[editingCell.field] === editingValue) {
      cancelEdit();
      return;
    }

    const originalValue = track[editingCell.field];
    track[editingCell.field] = editingValue; // Optimistic update

    try {
      const payload = {};
      payload[editingCell.field] = editingValue;
      
      const res = await fetch(`/api/tracks/${track.id}/tags`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });

      if (!res.ok) throw new Error("Save failed");
    } catch (e) {
      track[editingCell.field] = originalValue; // Rollback
      console.error(e);
    }

    cancelEdit();
  }

  function cancelEdit() {
    editingCell = { trackId: null, field: null };
    suggestions = [];
  }

  function handleKeyDown(e) {
    if (e.key === 'Enter') {
      if (suggestionIdx >= 0) {
        editingValue = suggestions[suggestionIdx];
        suggestions = [];
        suggestionIdx = -1;
      }
      saveEdit();
    } else if (e.key === 'Escape') {
      cancelEdit();
    } else if (e.key === 'ArrowDown' && suggestions.length > 0) {
      suggestionIdx = (suggestionIdx + 1) % suggestions.length;
    } else if (e.key === 'ArrowUp' && suggestions.length > 0) {
      suggestionIdx = (suggestionIdx - 1 + suggestions.length) % suggestions.length;
    }
  }

  async function handleOrganize(id) {
    try {
      const res = await fetch(`/api/tracks/organize?id=${encodeURIComponent(id)}`, { method: 'POST' });
      const data = await res.json();
      if (res.ok) alert("File organized to: " + data.newPath);
    } catch (e) { alert("Failed to organize"); }
  }

  async function handleRate(track, rating) {
    try {
      const res = await fetch('/api/tracks/rate', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: track.id, rating })
      });
      if (res.ok) track.rating = rating;
    } catch (e) { console.error(e); }
  }
</script>

<div class="tracks-view">
  <header class="section-header">
    <h1>Tracks</h1>
  </header>
  <div class="table-container">
    <table class="tracks-table">
      <thead>
        <tr>
          <th class="checkbox-cell">
            <input type="checkbox" onchange={toggleAll} checked={selectedIds.size === tracks.length && tracks.length > 0} />
          </th>
          <th onclick={() => toggleSort('title')} class="sortable">
            Title {sortConfig.field === 'title' ? (sortConfig.direction === 'asc' ? '↑' : '↓') : ''}
          </th>
          <th onclick={() => toggleSort('artist')} class="col-artist sortable">
            Artist {sortConfig.field === 'artist' ? (sortConfig.direction === 'asc' ? '↑' : '↓') : ''}
          </th>
          <th onclick={() => toggleSort('album')} class="col-album sortable">
            Album {sortConfig.field === 'album' ? (sortConfig.direction === 'asc' ? '↑' : '↓') : ''}
          </th>
          <th onclick={() => toggleSort('rating')} class="col-rating sortable">
            Rating {sortConfig.field === 'rating' ? (sortConfig.direction === 'asc' ? '↑' : '↓') : ''}
          </th>
          <th onclick={() => toggleSort('genre')} class="col-genre sortable">
            Genre {sortConfig.field === 'genre' ? (sortConfig.direction === 'asc' ? '↑' : '↓') : ''}
          </th>
        </tr>
      </thead>
      <tbody>
        {#each tracks as track}
          <tr 
            class="track-row" 
            class:active={currentPlaying && currentPlaying.id === track.id}
            class:selected={selectedIds.has(track.id)}
            onclick={(e) => handleRowClick(track, e)}
            ondblclick={() => onplay?.(track)}
            oncontextmenu={(e) => showContextMenu(track, e)}
            onkeydown={(e) => {
              if (e.key === 'Enter') onplay?.(track);
              if (e.key === ' ') { e.preventDefault(); toggleSelect(track.id); }
            }}
            tabindex="0"
          >
            <td class="checkbox-cell">
              <input type="checkbox" checked={selectedIds.has(track.id)} onchange={(e) => toggleSelect(track.id)} />
            </td>
            <td class="title-cell" ondblclick={(e) => startEdit(track, 'title', e)}>
              <div class="flex-align">
                <img class="mini-cover" src={`/api/cover/${track.id}`} alt="" />
                <div class="info-stack">
                  {#if editingCell.trackId === track.id && editingCell.field === 'title'}
                    <div class="inline-edit-wrap">
                      <input 
                        type="text" 
                        class="inline-input" 
                        value={editingValue} 
                        oninput={handleValueChange}
                        onkeydown={handleKeyDown}
                        onblur={saveEdit}
                        autoFocus 
                      />
                    </div>
                  {:else}
                    <span class="title-text">{track.title}</span>
                    <span class="mobile-subtitle">{track.artist}</span>
                  {/if}
                </div>
              </div>
            </td>
            <td class="col-artist" ondblclick={(e) => startEdit(track, 'artist', e)}>
              {#if editingCell.trackId === track.id && editingCell.field === 'artist'}
                <div class="inline-edit-wrap">
                  <input 
                    type="text" 
                    class="inline-input" 
                    value={editingValue} 
                    oninput={handleValueChange}
                    onkeydown={handleKeyDown}
                    onblur={saveEdit}
                    autoFocus 
                  />
                  {#if suggestions.length > 0}
                    <div class="suggestions-popover">
                      {#each suggestions as s, i}
                        <div class="suggestion-item" class:active={i === suggestionIdx} onmousedown={() => { editingValue = s; saveEdit(); }}>{s}</div>
                      {/each}
                    </div>
                  {/if}
                </div>
              {:else}
                {track.artist}
              {/if}
            </td>
            <td class="col-album" ondblclick={(e) => startEdit(track, 'album', e)}>
              {#if editingCell.trackId === track.id && editingCell.field === 'album'}
                <div class="inline-edit-wrap">
                  <input 
                    type="text" 
                    class="inline-input" 
                    value={editingValue} 
                    oninput={handleValueChange}
                    onkeydown={handleKeyDown}
                    onblur={saveEdit}
                    autoFocus 
                  />
                  {#if suggestions.length > 0}
                    <div class="suggestions-popover">
                      {#each suggestions as s, i}
                        <div class="suggestion-item" class:active={i === suggestionIdx} onmousedown={() => { editingValue = s; saveEdit(); }}>{s}</div>
                      {/each}
                    </div>
                  {/if}
                </div>
              {:else}
                {track.album}
              {/if}
            </td>
            <td class="col-rating">
              <StarRating rating={track.rating || 0} onchange={(rating) => handleRate(track, rating)} />
            </td>
            <td class="col-genre" ondblclick={(e) => startEdit(track, 'genre', e)}>
              {#if editingCell.trackId === track.id && editingCell.field === 'genre'}
                <div class="inline-edit-wrap">
                  <input 
                    type="text" 
                    class="inline-input" 
                    value={editingValue} 
                    oninput={handleValueChange}
                    onkeydown={handleKeyDown}
                    onblur={saveEdit}
                    autoFocus 
                  />
                </div>
              {:else}
                <span class="genre-tag">{track.genre || '-'}</span>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>

  <ContextMenu 
    bind:visible={contextMenu.visible}
    x={contextMenu.x}
    y={contextMenu.y}
    items={contextMenu.items}
    onclose={() => contextMenu.visible = false}
  />
</div>

<style>
  .tracks-view { width: 100%; padding: 0 0 2rem 0; }
  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 2.5rem;
  }
  h1 { font-size: 2.5rem; font-weight: 950; letter-spacing: -1.5px; margin: 0; color: var(--text-main); }
  .table-container { width: 100%; overflow-x: hidden; }
  .tracks-table { width: 100%; border-collapse: separate; border-spacing: 0 4px; font-size: 0.95rem; table-layout: fixed; }
  .tracks-table th { text-align: left; padding: 1rem; color: var(--text-muted); text-transform: uppercase; font-size: 0.75rem; font-weight: 800; letter-spacing: 1px; }
  .tracks-table th.sortable { cursor: pointer; user-select: none; }
  .tracks-table th.sortable:hover { color: var(--text-main); }
  
  .track-row { transition: all 0.2s; cursor: default; }
  .track-row td { transition: all 0.2s; border-top: 1px solid transparent; border-bottom: 1px solid transparent; }
  
  /* Modern rounded selection/hover effect */
  .track-row td:first-child { border-top-left-radius: 100px; border-bottom-left-radius: 100px; border-left: 1px solid transparent; }
  .track-row td:last-child { border-top-right-radius: 100px; border-bottom-right-radius: 100px; border-right: 1px solid transparent; }

  .track-row:hover td { background: var(--surface-container-high); border-color: var(--border-color); }
  
  .track-row.selected td { background: var(--primary-15); border-color: var(--primary-30); }
  .track-row.active .title-text { color: var(--primary); font-weight: 800; }
  
  .tracks-table td { padding: 0.8rem 1rem; color: var(--text-main); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  
  .checkbox-cell { width: 60px; text-align: center; }
  .title-cell { color: white; width: 40%; }
  
  .flex-align { display: flex; align-items: center; gap: 1rem; }
  .info-stack { display: flex; flex-direction: column; overflow: hidden; gap: 2px; }
  
  .mini-cover { width: 40px; height: 40px; border-radius: 10px; background: var(--surface-container-highest); object-fit: cover; box-shadow: 0 4px 10px rgba(0,0,0,0.2); }
  
  .title-text { font-weight: 700; color: var(--text-main); }
  .genre-tag { color: var(--secondary); font-size: 0.75rem; font-weight: 800; background: var(--secondary-10); padding: 0.25rem 0.6rem; border-radius: 6px; text-transform: uppercase; }
  
  .mobile-subtitle { display: none; font-size: 0.8rem; color: var(--text-muted); font-weight: 600; }

  /* Inline Editing Styles */
  .inline-edit-wrap { position: relative; width: 100%; display: flex; align-items: center; }
  .inline-input {
    width: 100%;
    background: var(--surface-container-highest);
    border: 1px solid var(--primary);
    color: white;
    padding: 0.4rem 0.8rem;
    border-radius: 8px;
    font-size: inherit;
    font-family: inherit;
    outline: none;
    box-shadow: 0 4px 12px rgba(0,0,0,0.3);
  }

  .suggestions-popover {
    position: absolute;
    top: 100%;
    left: 0;
    width: 100%;
    background: var(--surface-container-highest);
    border: 1px solid var(--border-color);
    border-radius: 12px;
    margin-top: 4px;
    box-shadow: 0 10px 30px rgba(0,0,0,0.5);
    z-index: 100;
    max-height: 200px;
    overflow-y: auto;
  }

  .suggestion-item {
    padding: 0.6rem 1rem;
    cursor: pointer;
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--text-main);
  }
  .suggestion-item:hover, .suggestion-item.active {
    background: var(--primary);
    color: white;
  }

  /* Table Column Widths */
  .col-artist { width: 20%; }
  .col-album { width: 20%; }
  .col-rating { width: 130px; }
  .col-genre { width: 15%; }

  @media (max-width: 1200px) {
    .col-genre, .col-rating { display: none; }
    .title-cell { width: 50%; }
  }

  @media (max-width: 768px) {
    .col-artist, .col-album { display: none; }
    .title-cell { width: 100%; }
    .mobile-subtitle { display: block; }
    .tracks-table th { padding: 0.5rem 1rem; }
    .tracks-table td { padding: 0.6rem 1rem; }
  }
</style>
