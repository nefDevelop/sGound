<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import { fade } from 'svelte/transition';

  export let playlist = { id: '', name: '' };
  
  let tracks = [];
  let loading = true;
  let error = null;

  $: if (playlist) {
    fetchPlaylistTracks();
  }

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
    if (tracks.length > 0) {
       // Dispatch event to parent to play first track and set queue
    }
  }
</script>

<div class="playlist-view" in:fade>
  {#if playlist}
    <div class="playlist-header">
      <div class="playlist-cover glass">
          <Icon name="list" size={64} color="rgba(255,255,255,0.2)" />
      </div>
      <div class="playlist-info">
        <span class="type-label">Playlist</span>
        <h1>{playlist.name}</h1>
        <p class="stats">{tracks.length} tracks</p>
        
        <div class="actions">
          <button class="play-btn" on:click={handlePlayAll}>
            <Icon name="play" size={20} color="black" />
            Play
          </button>
        </div>
      </div>
    </div>

    <div class="tracks-list">
      {#if loading}
        <p>Loading tracks...</p>
      {:else if error}
        <p class="error">{error}</p>
      {:else if tracks.length === 0}
        <div class="empty">
            <p>No tracks in this playlist.</p>
        </div>
      {:else}
        <div class="track-grid-header">
          <span>#</span>
          <span>Title</span>
          <span>Album</span>
          <span>Duration</span>
        </div>
        {#each tracks as track, i}
          <div class="track-row" on:dblclick={() => console.log("Play", track)} role="button" tabindex="0">
            <span class="num">{i + 1}</span>
            <div class="title-cell">
              <span class="title">{track.title}</span>
              <span class="artist">{track.artist}</span>
            </div>
            <span class="album">{track.album}</span>
            <span class="dur">3:45</span>
          </div>
        {/each}
      {/if}
    </div>
  {/if}
</div>

<style>
  .playlist-view {
    padding: 2rem;
  }

  .playlist-header {
    display: flex;
    gap: 2rem;
    align-items: flex-end;
    margin-bottom: 3rem;
  }

  .playlist-cover {
    width: 230px;
    height: 230px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: linear-gradient(135deg, #333 0%, #111 100%);
    border-radius: 12px;
    box-shadow: 0 20px 50px rgba(0,0,0,0.4);
  }

  .playlist-info h1 {
    font-size: 4rem;
    margin: 0.5rem 0;
    font-weight: 900;
  }

  .type-label {
    text-transform: uppercase;
    font-size: 0.8rem;
    font-weight: 700;
    letter-spacing: 1px;
  }

  .stats {
    color: var(--text-muted);
    margin-bottom: 1.5rem;
  }

  .play-btn {
    background: var(--accent);
    color: #000;
    border: none;
    padding: 0.8rem 2rem;
    border-radius: 30px;
    font-weight: 700;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    cursor: pointer;
    transition: transform 0.2s;
  }
  .play-btn:hover { transform: scale(1.05); }

  .track-grid-header {
    display: grid;
    grid-template-columns: 40px 1fr 1fr 100px;
    padding: 1rem;
    border-bottom: 1px solid rgba(255,255,255,0.05);
    color: var(--text-muted);
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 1px;
  }

  .track-row {
    display: grid;
    grid-template-columns: 40px 1fr 1fr 100px;
    padding: 0.8rem 1rem;
    border-radius: 8px;
    align-items: center;
    cursor: pointer;
    transition: background 0.2s;
  }
  .track-row:hover { background: rgba(255,255,255,0.05); }

  .title-cell { display: flex; flex-direction: column; }
  .title { font-weight: 600; }
  .artist { font-size: 0.85rem; color: var(--text-muted); }
  .album { color: var(--text-muted); }
  .num { color: var(--text-muted); }
</style>
