<script>
  import { createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';
  
  export let tracks = [];
  export let currentPlaying = null;
  
  const dispatch = createEventDispatcher();
  let selectedIds = new Set();

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
</script>

<div class="tracks-view">
  <table class="tracks-table">
    <thead>
      <tr>
        <th class="checkbox-cell">
          <input type="checkbox" on:change={toggleAll} checked={selectedIds.size === tracks.length && tracks.length > 0} />
        </th>
        <th>#</th>
        <th>Title</th>
        <th>Artist</th>
        <th>Album</th>
        <th>Genre</th>
        <th>Actions</th>
      </tr>
    </thead>
    <tbody>
      {#each tracks as track}
        <tr 
          class="track-row" 
          class:active={currentPlaying?.id === track.id}
          class:selected={selectedIds.has(track.id)}
          on:dblclick={() => dispatch('play', track)}
        >
          <td class="checkbox-cell">
            <input type="checkbox" checked={selectedIds.has(track.id)} on:change={(e) => toggleSelect(track.id, e)} />
          </td>
          <td class="num">{track.trackNum || '-'}</td>
          <td class="title-cell">
            <div class="flex-align">
              <!-- svelte-ignore a11y-missing-attribute -->
              <img class="mini-cover" src={`/api/cover?id=${encodeURIComponent(track.id)}`} on:error={(e) => (e.currentTarget.style.display='none')} />
              <span class="title-text">{track.title}</span>
            </div>
          </td>
          <td>{track.artist}</td>
          <td>{track.album}</td>
          <td><span class="genre-tag">{track.genre || '-'}</span></td>
          <td class="actions">
            <button class="btn-icon" on:click={() => dispatch('play', track)} title="Play"><Icon name="play" size={14} /></button>
            <button class="btn-icon" on:click={() => dispatch('edit', track)} title="Edit Tags"><Icon name="edit" size={14} /></button>
            <button class="btn-icon" on:click={() => handleOrganize(track.id)} title="Organize File"><Icon name="folder" size={14} /></button>
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
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
