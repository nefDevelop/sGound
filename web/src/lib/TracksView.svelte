<script>
  import Icon from './Icon.svelte';
  import ContextMenu from './ContextMenu.svelte';
  import StarRating from './StarRating.svelte';
  
  let { 
    tracks = $bindable([]), 
    currentPlaying = null, 
    onplay, 
    onedit, 
    onselectionChange, 
    onqueue, 
    ondelete 
  } = $props();
  
  let selectedIds = $state(new Set());

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
        { label: 'Organize file', icon: 'folder', action: () => handleOrganize(track.id) },
        { divider: true },
        { label: 'Copy title', icon: 'fileText', action: () => navigator.clipboard.writeText(track.title) },
        { label: 'Remove from library', icon: 'xCircle', color: 'danger', action: () => ondelete?.(track) }
      ]
    };
  }

  function toggleSelect(id, event) {
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
  <div class="table-container">
    <table class="tracks-table">
      <thead>
        <tr>
          <th class="checkbox-cell">
            <input type="checkbox" onchange={toggleAll} checked={selectedIds.size === tracks.length && tracks.length > 0} />
          </th>
          <th class="col-num">#</th>
          <th>Title</th>
          <th class="col-artist">Artist</th>
          <th class="col-rating">Rating</th>
          <th class="col-album">Album</th>
          <th class="col-genre">Genre</th>
        </tr>
      </thead>
      <tbody>
        {#each tracks as track}
          <tr 
            class="track-row" 
            class:active={currentPlaying && currentPlaying.id === track.id}
            class:selected={selectedIds.has(track.id)}
            ondblclick={() => onplay?.(track)}
            oncontextmenu={(e) => showContextMenu(track, e)}
          >
            <td class="checkbox-cell">
              <input type="checkbox" checked={selectedIds.has(track.id)} onchange={(e) => toggleSelect(track.id, e)} />
            </td>
            <td class="num col-num">{track.trackNum || '-'}</td>
            <td class="title-cell">
              <div class="flex-align">
                <img 
                  class="mini-cover" 
                  src={`/api/cover/${track.id}`} 
                  alt="" 
                  onerror={(e) => { 
                    const target = e.currentTarget;
                    if (target instanceof HTMLImageElement) target.style.display='none'; 
                  }} 
                />
                <div class="info-stack">
                  <span class="title-text">{track.title}</span>
                  <span class="mobile-subtitle">{track.artist}</span>
                </div>
              </div>
            </td>
            <td class="col-artist">{track.artist}</td>
            <td class="col-rating">
              <StarRating rating={track.rating || 0} onchange={(rating) => handleRate(track, rating)} />
            </td>
            <td class="col-album">{track.album}</td>
            <td class="col-genre"><span class="genre-tag">{track.genre || '-'}</span></td>
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
  .table-container { width: 100%; overflow-x: hidden; }
  .tracks-table { width: 100%; border-collapse: collapse; font-size: 0.95rem; table-layout: fixed; }
  .tracks-table th { text-align: left; padding: 1rem; color: var(--text-muted); border-bottom: 1px solid var(--border-color); text-transform: uppercase; font-size: 0.75rem; font-weight: 800; letter-spacing: 1px; }
  
  .track-row { transition: background 0.2s; cursor: default; }
  .track-row:hover { background: var(--surface-container-high); }
  .track-row.selected { background: var(--primary-10); }
  .track-row.active .title-text { color: var(--primary); font-weight: 800; }
  
  .tracks-table td { padding: 0.8rem 1rem; border-bottom: 1px solid var(--border-color); color: var(--text-main); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  
  .checkbox-cell { width: 60px; text-align: center; }
  .col-num { width: 50px; color: var(--text-muted); font-weight: 700; }
  .title-cell { color: white; width: 40%; }
  
  .flex-align { display: flex; align-items: center; gap: 1rem; }
  .info-stack { display: flex; flex-direction: column; overflow: hidden; gap: 2px; }
  
  .mini-cover { width: 40px; height: 40px; border-radius: 10px; background: var(--surface-container-highest); object-fit: cover; box-shadow: 0 4px 10px rgba(0,0,0,0.2); }
  
  .title-text { font-weight: 700; color: var(--text-main); }
  .genre-tag { color: var(--secondary); font-size: 0.75rem; font-weight: 800; background: var(--secondary-10); padding: 0.25rem 0.6rem; border-radius: 6px; text-transform: uppercase; }
  
  
  .mobile-subtitle { display: none; font-size: 0.8rem; color: var(--text-muted); font-weight: 600; }

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
    .col-artist, .col-album, .col-num { display: none; }
    .title-cell { width: 100%; }
    .mobile-subtitle { display: block; }
    .tracks-table th { padding: 0.5rem 1rem; }
    .tracks-table td { padding: 0.6rem 1rem; }
  }
</style>
