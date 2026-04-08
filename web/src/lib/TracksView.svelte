<script>
  import { createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';
  import ContextMenu from './ContextMenu.svelte';
  import StarRating from './StarRating.svelte';
  
  export let tracks = [];
  /** @type {any} */
  export let currentPlaying = null;
  
  const dispatch = createEventDispatcher();
  let selectedIds = new Set();

  let contextMenu = {
    visible: false,
    x: 0,
    y: 0,
    track: null,
    items: []
  };

  function showContextMenu(track, e) {
    e.preventDefault();
    contextMenu = {
      visible: true,
      x: e.clientX,
      y: e.clientY,
      track,
      items: [
        { label: 'Reproducir', icon: 'play', action: () => dispatch('play', { track, tracks }) },
        { label: 'Añadir a cola', icon: 'plus', action: () => dispatch('queue', track) },
        { divider: true },
        { label: 'Editar etiquetas', icon: 'edit', action: () => dispatch('edit', track) },
        { label: 'Organizar archivo', icon: 'folder', action: () => handleOrganize(track.id) },
        { divider: true },
        { label: 'Copiar título', icon: 'fileText', action: () => navigator.clipboard.writeText(track.title) },
        { label: 'Eliminar de biblioteca', icon: 'xCircle', color: 'danger', action: () => dispatch('delete', track) }
      ]
    };
  }

  function toggleSelect(id, event) {
    if (selectedIds.has(id)) {
      selectedIds.delete(id);
    } else {
      selectedIds.add(id);
    }
    selectedIds = selectedIds; 
    dispatch('selectionChange', Array.from(selectedIds).map(sid => tracks.find(t => t.id === sid)));
  }

  function toggleAll() {
    if (selectedIds.size === tracks.length) {
      selectedIds.clear();
    } else {
      tracks.forEach(t => selectedIds.add(t.id));
    }
    selectedIds = selectedIds;
    dispatch('selectionChange', Array.from(selectedIds).map(sid => tracks.find(t => t.id === sid)));
  }

  async function handleOrganize(id) {
    try {
      const res = await fetch(`/api/tracks/organize?id=${encodeURIComponent(id)}`, { method: 'POST' });
      const data = await res.json();
      if (res.ok) {
        alert("File organized to: " + data.newPath);
      }
    } catch (e) { alert("Failed to organize"); }
  }

  async function handleRate(track, rating) {
    try {
      const res = await fetch('/api/tracks/rate', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: track.id, rating })
      });
      if (res.ok) {
        track.rating = rating;
        tracks = tracks; // trigger reactivity
      }
    } catch (e) { console.error(e); }
  }
</script>

<div class="tracks-view">
  <table class="tracks-table">
    <thead>
      <tr>
        <th class="checkbox-cell">
          <input type="checkbox" onchange={toggleAll} checked={selectedIds.size === tracks.length && tracks.length > 0} />
        </th>
        <th>#</th>
        <th>Title</th>
        <th>Artist</th>
        <th>Rating</th>
        <th>Album</th>
        <th>Genre</th>
        <th>Actions</th>
      </tr>
    </thead>
    <tbody>
      {#each tracks as track}
        <tr 
          class="track-row" 
          class:active={currentPlaying && currentPlaying.id === track.id}
          class:selected={selectedIds.has(track.id)}
          ondblclick={() => dispatch('play', { track, tracks })}
          oncontextmenu={(e) => showContextMenu(track, e)}
        >
          <td class="checkbox-cell">
            <input type="checkbox" checked={selectedIds.has(track.id)} onchange={(e) => toggleSelect(track.id, e)} />
          </td>
          <td class="num">{track.trackNum || '-'}</td>
          <td class="title-cell">
            <div class="flex-align">
              <!-- svelte-ignore a11y-missing-attribute -->
              <img class="mini-cover" src={`/api/cover/${track.id}`} onerror={(e) => { if (e.currentTarget instanceof HTMLImageElement) e.currentTarget.style.display='none'; }} />
              <span class="title-text">{track.title}</span>
            </div>
          </td>
          <td>{track.artist}</td>
          <td>
            <StarRating rating={track.rating || 0} on:change={(e) => handleRate(track, e.detail)} />
          </td>
          <td>{track.album}</td>
          <td><span class="genre-tag">{track.genre || '-'}</span></td>
          <td class="actions">
            <button class="btn-icon" onclick={() => dispatch('play', { track, tracks })} title="Play"><Icon name="play" size={14} /></button>
            <button class="btn-icon" onclick={() => dispatch('edit', track)} title="Edit Tags"><Icon name="edit" size={14} /></button>
            <button class="btn-icon" onclick={() => handleOrganize(track.id)} title="Organize File"><Icon name="folder" size={14} /></button>
          </td>
        </tr>
      {/each}
    </tbody>
  </table>

  <ContextMenu 
    bind:visible={contextMenu.visible}
    x={contextMenu.x}
    y={contextMenu.y}
    items={contextMenu.items}
    on:close={() => contextMenu.visible = false}
  />
</div>

<style>
  .tracks-view { width: 100%; overflow-x: auto; }
  .tracks-table { width: 100%; border-collapse: collapse; font-size: 0.95rem; }
  .tracks-table th { text-align: left; padding: 1rem; color: #64748b; border-bottom: 1px solid rgba(255,255,255,0.05); text-transform: uppercase; font-size: 0.8rem; }
  .track-row { transition: background 0.2s; cursor: default; }
  .track-row:hover { background: rgba(255, 255, 255, 0.03); }
  .track-row.selected { background: rgba(255, 51, 102, 0.1); }
  .track-row.active .title-text { color: #ff3366; font-weight: 600; }
  .tracks-table td { padding: 0.8rem 1rem; border-bottom: 1px solid rgba(255,255,255,0.02); color: #cbd5e1; }
  .checkbox-cell { width: 40px; text-align: center; }
  .num { width: 40px; color: #64748b; }
  .title-cell { color: white; }
  .flex-align { display: flex; align-items: center; gap: 1rem; }
  .mini-cover { width: 32px; height: 32px; border-radius: 4px; background: #1e293b; object-fit: cover; }
  .genre-tag { color: #ff3366; font-size: 0.85rem; }
  .btn-icon { background: transparent; border: none; color: #64748b; font-size: 1rem; padding: 0.4rem; border-radius: 4px; cursor: pointer; }
  .btn-icon:hover { color: white; background: rgba(255,255,255,0.1); }
</style>
