<script>
  import { onMount } from 'svelte';
  import TagEditor from './lib/TagEditor.svelte';
  
  let artists = [];
  let error = null;

  let isTagEditorOpen = false;
  let trackToEdit = { id: '', title: '', artist: '', album: '' };

  onMount(async () => {
    try {
      const res = await fetch('/api/artists');
      if (!res.ok) throw new Error("Failed to fetch library");
      artists = await res.json() || [];
    } catch (e) {
      error = e.message;
    }
  });

  function openEditor(artist) {
    trackToEdit = { id: 'mock-1', title: 'Sample Track', artist: artist.name, album: 'Sample Album' };
    isTagEditorOpen = true;
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
      <h1>Your Library</h1>
      <button class="primary-btn">Scan Library</button>
    </header>

    <div class="library-grid">
      {#if error}
        <p class="error">{error}</p>
      {:else if artists.length === 0}
        <div class="empty-state">
          <p>Your library is empty. Please run a scan.</p>
        </div>
      {:else}
        {#each artists as artist}
          <!-- svelte-ignore a11y-click-events-have-key-events -->
          <!-- svelte-ignore a11y-no-static-element-interactions -->
          <div class="artist-card glass" on:click={() => openEditor(artist)}>
            <div class="avatar"></div>
            <h3>{artist.name}</h3>
          </div>
        {/each}
      {/if}
    </div>
  </section>

  <!-- Player Bar -->
  <footer class="player glass">
    <div class="now-playing">
      <div class="cover"></div>
      <div class="track-info">
        <span class="title">Unknown Track</span>
        <span class="artist">Select a song to play</span>
      </div>
    </div>
    
    <div class="controls">
      <button class="icon-btn">⏪</button>
      <button class="icon-btn play">▶</button>
      <button class="icon-btn">⏩</button>
    </div>
    
    <div class="extras">
      <input type="range" min="0" max="100" value="50" class="volume-slider">
    </div>
  </footer>
</main>

<TagEditor 
  bind:isOpen={isTagEditorOpen} 
  track={trackToEdit} 
  on:close={() => { /* refresh logic here if needed */ }} 
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
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 1.5rem;
  }

  .artist-card {
    padding: 1.5rem;
    border-radius: 16px;
    text-align: center;
    transition: transform 0.3s ease, box-shadow 0.3s ease;
    cursor: pointer;
  }

  .artist-card:hover {
    transform: translateY(-5px);
    box-shadow: 0 10px 30px rgba(0,0,0,0.5);
  }

  .avatar {
    width: 120px;
    height: 120px;
    border-radius: 50%;
    background: #334155;
    margin: 0 auto 1rem;
  }

  .artist-card h3 {
    margin: 0;
    font-size: 1.1rem;
  }

  /* Player */
  .player {
    grid-area: player;
    border-top: 1px solid var(--border-color);
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 2rem;
    z-index: 10;
  }

  .now-playing {
    display: flex;
    align-items: center;
    gap: 1rem;
    width: 300px;
  }

  .cover {
    width: 56px;
    height: 56px;
    background: #334155;
    border-radius: 8px;
  }

  .track-info {
    display: flex;
    flex-direction: column;
  }

  .title {
    font-weight: 600;
    font-size: 1.1rem;
  }

  .artist {
    color: var(--text-muted);
    font-size: 0.9rem;
  }

  .controls {
    display: flex;
    align-items: center;
    gap: 1.5rem;
  }

  .icon-btn {
    font-size: 1.5rem;
    color: var(--text-muted);
  }

  .icon-btn:hover {
    color: var(--text-main);
  }

  .icon-btn.play {
    font-size: 2rem;
    color: var(--text-main);
  }

  .error { color: #ef4444; }
  .empty-state { color: var(--text-muted); text-align: center; grid-column: 1/-1; padding: 4rem 0;}
</style>
