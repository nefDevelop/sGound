<script>
  import { onMount } from 'svelte';
  import TagEditor from './lib/TagEditor.svelte';
  
  let tracks = [];
  let error = null;

  let isTagEditorOpen = false;
  let trackToEdit = { id: '', title: '', artist: '', album: '' };

  let currentPlaying = null;
  let audioRef = null;

  onMount(async () => {
    fetchTracks();
  });

  async function fetchTracks() {
    try {
      const res = await fetch('/api/tracks');
      if (!res.ok) throw new Error("Failed to fetch tracks");
      tracks = await res.json() || [];
    } catch (e) {
      error = e.message;
    }
  }

  function openEditor(track) {
    trackToEdit = { ...track };
    isTagEditorOpen = true;
  }

  async function deleteTrack(id) {
    if (!confirm("Are you sure you want to delete this track from DB?")) return;
    try {
      const res = await fetch(`/api/tracks/${id}`, { method: 'DELETE' });
      if (!res.ok) throw new Error("Error deleting record");
      tracks = tracks.filter(t => t.id !== id);
    } catch(e) {
      alert(e);
    }
  }

  function playTrack(track) {
    currentPlaying = track;
    if (audioRef) {
      audioRef.src = `/api/stream/${track.id}`;
      audioRef.play();
    }
  }
</script>

<main class="app-layout">
  <!-- Sidebar -->
  <aside class="sidebar glass">
    <div class="brand">
      <div class="logo"></div>
      <h2>Sgound</h2>
    </div>
    
    <nav>
      <button class="nav-item active">Library</button>
      <button class="nav-item">Playlists</button>
      <button class="nav-item">Settings</button>
    </nav>
  </aside>

  <!-- Main Content -->
  <section class="content">
    <header class="top-bar">
      <h1>Your Tracks</h1>
      <button class="primary-btn">Scan Library</button>
    </header>

    <div class="library-grid">
      {#if error}
        <p class="error">{error}</p>
      {:else if tracks.length === 0}
        <div class="empty-state">
          <p>Your library is empty. Add MP3/FLACs to ./music directory.</p>
        </div>
      {:else}
        <table class="tracks-table">
          <thead>
            <tr>
              <th>#</th>
              <th>Title</th>
              <th>Artist</th>
              <th>Album</th>
              <th>Genre</th>
              <th>Year</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {#each tracks as track}
              <!-- svelte-ignore a11y-click-events-have-key-events -->
              <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
              <tr class="glass-row" on:dblclick={() => playTrack(track)}>
                <td>{track.trackNum || '-'}</td>
                <td class="bold">{track.title}</td>
                <td>{track.artist}</td>
                <td>{track.album}</td>
                <td style="color:var(--primary-color)">{track.genre || '-'}</td>
                <td style="opacity: 0.6">{track.year || '-'}</td>
                <td class="actions-cell">
                  <button class="icon-btn-small" on:click={() => playTrack(track)}>▶</button>
                  <button class="icon-btn-small" on:click={() => openEditor(track)}>✏️</button>
                  <button class="icon-btn-small delete" on:click={() => deleteTrack(track.id)}>🗑️</button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>
  </section>

  <!-- Player Bar -->
  <footer class="player glass">
    <div class="now-playing">
      {#if currentPlaying}
        <img class="cover" src={`/api/cover/${currentPlaying.id}`} alt="Artwork" />
      {:else}
        <div class="cover"></div>
      {/if}
      <div class="track-info">
        <span class="title">{currentPlaying ? currentPlaying.title : 'No Track'}</span>
        <span class="artist">{currentPlaying ? currentPlaying.artist : 'Select a song'}</span>
      </div>
    </div>
    
    <div class="controls">
      <!-- Reproductor HTML5 Nativo embebido -->
      <audio bind:this={audioRef} controls class="native-audio"></audio>
    </div>
    
    <div class="extras">
      <!-- placeholder -->
    </div>
  </footer>
</main>

<TagEditor 
  bind:isOpen={isTagEditorOpen} 
  track={trackToEdit} 
  on:close={() => { fetchTracks() }} 
/>

<style>
  .app-layout {
    display: grid;
    grid-template-areas: 
      "sidebar content"
      "player player";
    grid-template-columns: 240px 1fr;
    grid-template-rows: 1fr 90px;
    height: 100vh;
    overflow: hidden;
  }

  /* Sidebar */
  .sidebar {
    grid-area: sidebar;
    border-right: 1px solid var(--border-color);
    padding: 2rem 1rem;
    display: flex;
    flex-direction: column;
    gap: 2rem;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 0 1rem;
  }

  .logo {
    width: 32px;
    height: 32px;
    background: linear-gradient(135deg, var(--accent-color), #ff6b6b);
    border-radius: 8px;
  }

  .brand h2 {
    margin: 0;
    font-size: 1.5rem;
    font-weight: 700;
    letter-spacing: -0.5px;
  }

  .nav-item {
    width: 100%;
    text-align: left;
    padding: 1rem 1.5rem;
    border-radius: 12px;
    font-size: 1.1rem;
    color: var(--text-muted);
    margin-bottom: 0.5rem;
  }

  .nav-item:hover, .nav-item.active {
    background: rgba(255, 255, 255, 0.05);
    color: var(--text-main);
  }

  .nav-item.active {
    font-weight: 600;
  }

  /* Main Content */
  .content {
    grid-area: content;
    padding: 2rem 3rem;
    overflow-y: auto;
  }

  .top-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 3rem;
  }

  .top-bar h1 {
    font-size: 2.5rem;
    margin: 0;
    font-weight: 700;
  }

  .library-grid {
    width: 100%;
    margin-top: 1rem;
  }

  .tracks-table {
    width: 100%;
    border-collapse: collapse;
    text-align: left;
    background: rgba(26, 31, 53, 0.4);
    border-radius: 12px;
    overflow: hidden;
    backdrop-filter: blur(8px);
  }

  .tracks-table th {
    padding: 1rem 1.5rem;
    color: var(--text-muted);
    font-weight: 600;
    border-bottom: 1px solid var(--border-color);
  }

  .tracks-table td {
    padding: 1rem 1.5rem;
    color: var(--text-main);
    border-bottom: 1px solid rgba(255,255,255,0.03);
  }

  .glass-row {
    transition: background 0.2s ease;
    cursor: pointer;
  }

  .glass-row:hover {
    background: rgba(255, 51, 102, 0.1);
  }

  .bold { font-weight: 600; }

  .actions-cell {
    display: flex;
    gap: 0.8rem;
  }

  .icon-btn-small {
    background: rgba(255,255,255,0.05);
    border-radius: 6px;
    padding: 0.4rem 0.6rem;
    font-size: 1rem;
  }

  .icon-btn-small:hover {
    background: rgba(255, 255, 255, 0.15);
    color: white;
  }
  .icon-btn-small.delete:hover {
    background: #ef4444;
  }

  .native-audio {
    height: 40px;
    border-radius: 8px;
    outline: none;
    filter: invert(0.9) hue-rotate(180deg);
  }

  .error { color: #ef4444; }
  .empty-state { color: var(--text-muted); text-align: center; grid-column: 1/-1; padding: 4rem 0;}
</style>
