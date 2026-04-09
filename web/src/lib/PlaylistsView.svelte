<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import { fade } from 'svelte/transition';
  import TracksView from './TracksView.svelte';

  let { playlist = { id: '', name: '' }, currentPlaying = null, onplay, onedit, onselectionChange, onqueue, ondelete } = $props();
  
  let tracks = $state([]);
  let loading = $state(true);
  let error = $state(null);

  $effect(() => {
    if (playlist?.id) fetchPlaylistTracks();
  });

  async function fetchPlaylistTracks() {
    loading = true;
    try {
      const res = await fetch(`/api/playlists/${playlist.id}/tracks`);
      if (!res.ok) throw new Error("Could not load playlist tracks");
      tracks = await res.json();
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function handlePlayAll() {
    if (tracks.length > 0) onplay?.(tracks[0]);
  }
</script>

<div class="playlist-view" in:fade>
  {#if playlist}
    <div class="playlist-header">
      <div class="playlist-cover">
          <Icon name="list" size={80} />
      </div>
      <div class="playlist-info">
        <span class="type-label">Playlist</span>
        <h1>{playlist.name}</h1>
        <div class="meta-row">
          <p class="stats">{tracks.length} tracks</p>
          <button class="play-btn" onclick={handlePlayAll}>
            <Icon name="play" size={20} color="black" />
            Play All
          </button>
        </div>
      </div>
    </div>

    <div class="tracks-list">
      {#if loading}
        <div class="status">Sourcing tracks...</div>
      {:else if error}
        <p class="error">{error}</p>
      {:else if tracks.length === 0}
        <div class="empty">
            <Icon name="music" size={48} />
            <p>This playlist is empty.</p>
        </div>
      {:else}
        <TracksView 
          {tracks} 
          {currentPlaying} 
          onplay={onplay}
          onedit={onedit}
          onselectionChange={onselectionChange}
          onqueue={onqueue}
          ondelete={ondelete}
        />
      {/if}
    </div>
  {/if}
</div>

<style>
  .playlist-view { width: 100%; padding: 0 0 2rem 0; }
  
  .playlist-header {
    display: flex;
    gap: 3rem;
    align-items: center;
    margin-bottom: 4rem;
  }

  .playlist-cover {
    width: 240px; height: 240px;
    display: flex; align-items: center; justify-content: center;
    background: var(--surface-container-high);
    border-radius: 32px;
    box-shadow: 0 30px 60px rgba(0,0,0,0.4);
    color: var(--primary);
    border: 1px solid var(--border-color);
  }

  h1 { 
    font-family: var(--font-headline); 
    font-size: clamp(2.5rem, 6vw, 4rem); 
    margin: 0; 
    letter-spacing: -0.04em; 
    font-weight: 950;
    line-height: 1;
  }
  .type-label {
    text-transform: uppercase;
    font-size: 0.8rem;
    font-weight: 900;
    letter-spacing: 2px;
    color: var(--primary);
  }

  .meta-row { display: flex; align-items: center; gap: 2rem; }

  .stats {
    color: var(--text-muted);
    font-weight: 800;
    font-size: 1.1rem;
    margin: 0;
  }

  .play-btn {
    background: var(--primary);
    color: white;
    border: none;
    padding: 1.2rem 3rem;
    border-radius: 100px;
    font-weight: 900;
    display: flex;
    align-items: center;
    gap: 0.8rem;
    cursor: pointer;
    transition: all 0.3s cubic-bezier(0.2, 0, 0, 1);
    box-shadow: 0 10px 30px var(--primary-20);
  }
  .play-btn:hover { transform: translateY(-4px); box-shadow: 0 15px 40px var(--primary-30); }

  .tracks-list { margin-top: 1rem; }

  .status { padding: 4rem; text-align: center; color: var(--text-muted); font-weight: 700; }

  .empty {
    text-align: center;
    padding: 4rem;
    color: var(--text-muted);
    background: var(--surface-container-low);
    border-radius: 32px;
    border: 1px dashed var(--border-color);
    display: flex; flex-direction: column; align-items: center; gap: 1rem;
  }

  @media (max-width: 900px) {
    .playlist-header { 
      flex-direction: row; 
      gap: 1.5rem; 
      align-items: center; 
      text-align: left; 
      margin-bottom: 2rem;
    }
    .playlist-cover { 
      width: 100px; 
      height: 100px; 
      border-radius: 20px; 
      flex-shrink: 0;
    }
    .playlist-cover :global(svg) { width: 40px; height: 40px; }
    h1 { font-size: 1.8rem; }
    .meta-row { flex-direction: row; gap: 1rem; align-items: center; margin-top: 0.5rem; }
    .play-btn { padding: 0.7rem 1.2rem; font-size: 0.85rem; }
    .stats { font-size: 0.9rem; }
    .type-label { font-size: 0.65rem; }
  }
</style>

