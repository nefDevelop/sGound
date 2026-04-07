<script>
  import { onMount } from 'svelte';
  import TagEditor from './lib/TagEditor.svelte';
  import MassiveTagEditor from './lib/MassiveTagEditor.svelte';
  import MiniPlayer from './lib/MiniPlayer.svelte';
  import TracksView from './lib/TracksView.svelte';
  import AlbumsGrid from './lib/AlbumsGrid.svelte';
  import Dashboard from './lib/Dashboard.svelte';
  import Settings from './lib/Settings.svelte';
  
  let currentView = "dashboard"; 
  let tracks = [];
  let error = null;
  let searchQuery = "";

  let isTagEditorOpen = false;
  let isMassiveEditorOpen = false;
  let trackToEdit = { id: '', title: '', artist: '', album: '' };
  let selectedTracks = [];

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

  async function handleSearch() {
    if (!searchQuery) {
      fetchTracks();
      return;
    }
    try {
      const res = await fetch(`/api/search?q=${encodeURIComponent(searchQuery)}`);
      const data = await res.json();
      tracks = data.tracks || [];
    } catch (e) {
      console.error("Search failed", e);
    }
  }

  async function triggerScan() {
    try {
      const res = await fetch('/api/scan', { method: 'POST' });
      if (res.ok) {
        alert("Library scan started!");
        setTimeout(fetchTracks, 2000);
      }
    } catch (e) {
      alert("Scan failed");
    }
  }

  function openEditor(track) {
    trackToEdit = { ...track };
    isTagEditorOpen = true;
  }

  function handlePlay(track) {
    currentPlaying = track;
    if (audioRef) {
      audioRef.src = `/api/stream/${track.id}`;
      audioRef.play();
    }
  }

  function handleSelectionChange(selected) {
    selectedTracks = selected;
  }

  async function selectAlbum(albumId) {
    try {
      const res = await fetch(`/api/albums/${albumId}/tracks`);
      tracks = await res.json() || [];
      currentView = "library";
    } catch (e) {
      console.error(e);
    }
  }
</script>

<main class="app-layout">
  <aside class="sidebar">
    <div class="brand">
      <div class="logo"></div>
      <h2>Sgound</h2>
    </div>
    
    <nav>
      <button class="nav-item" class:active={currentView === 'dashboard'} on:click={() => currentView = 'dashboard'}>
        Dashboard
      </button>
      <button class="nav-item" class:active={currentView === 'library'} on:click={() => currentView = 'library'}>
        Library
      </button>
      <button class="nav-item" class:active={currentView === 'albums'} on:click={() => currentView = 'albums'}>
        Albums
      </button>
      <button class="nav-item" class:active={currentView === 'settings'} on:click={() => currentView = 'settings'}>
        Settings
      </button>
    </nav>
  </aside>

  <section class="content">
    <header class="top-bar">
      <div class="search-box">
        <input type="text" placeholder="Search music..." bind:value={searchQuery} on:input={handleSearch} />
      </div>
      <div class="actions">
        {#if selectedTracks.length > 1}
          <button class="bulk-btn" on:click={() => isMassiveEditorOpen = true}>
            Edit ({selectedTracks.length})
          </button>
        {/if}
        <button class="primary-btn" on:click={triggerScan}>Scan Library</button>
      </div>
    </header>

    <div class="scroll-view">
      {#if error}
        <p class="error">{error}</p>
      {:else if currentView === 'dashboard'}
        <Dashboard on:play={(e) => handlePlay(e.detail)} />
      {:else if currentView === 'library'}
        {#if tracks.length === 0}
          <div class="empty-state">
            <p>No tracks found. Click Scan Library to index your music.</p>
          </div>
        {:else}
          <TracksView 
            {tracks} 
            {currentPlaying} 
            on:play={(e) => handlePlay(e.detail)} 
            on:edit={(e) => openEditor(e.detail)}
            on:selectionChange={(e) => handleSelectionChange(e.detail)}
          />
        {/if}
      {:else if currentView === 'albums'}
        <AlbumsGrid on:selectAlbum={(e) => selectAlbum(e.detail)} />
      {:else if currentView === 'settings'}
        <Settings />
      {/if}
    </div>
  </section>

  <MiniPlayer {currentPlaying} bind:audioRef />
</main>

<TagEditor 
  bind:isOpen={isTagEditorOpen} 
  track={trackToEdit} 
  on:close={fetchTracks} 
/>

<MassiveTagEditor 
  bind:isOpen={isMassiveEditorOpen} 
  {selectedTracks} 
  on:close={fetchTracks} 
/>

<style>
  .app-layout {
    display: grid;
    grid-template-areas: "sidebar content" "player player";
    grid-template-columns: 240px 1fr;
    grid-template-rows: 1fr 90px;
    height: 100vh;
    background: #0f111a;
    color: #cbd5e1;
  }

  .sidebar {
    grid-area: sidebar;
    background: #090a10;
    padding: 2rem 1rem;
    border-right: 1px solid rgba(255,255,255,0.05);
  }

  .logo { width: 32px; height: 32px; background: #ff3366; border-radius: 8px; margin-bottom: 2rem; }

  .nav-item {
    display: block;
    width: 100%;
    padding: 0.8rem 1rem;
    background: transparent;
    border: none;
    color: #64748b;
    text-align: left;
    border-radius: 8px;
    cursor: pointer;
    font-weight: 600;
    font-size: 0.95rem;
    margin-bottom: 0.2rem;
  }

  .nav-item.active { background: rgba(255, 51, 102, 0.1); color: #ff3366; }

  .content { grid-area: content; display: flex; flex-direction: column; overflow: hidden; }

  .top-bar {
    padding: 1.5rem 2rem;
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: rgba(15, 17, 26, 0.8);
    backdrop-filter: blur(10px);
  }

  .search-box input {
    background: #1e293b;
    border: none;
    padding: 0.6rem 1.2rem;
    border-radius: 20px;
    color: white;
    width: 300px;
  }

  .primary-btn { background: #ff3366; color: white; border: none; padding: 0.6rem 1.2rem; border-radius: 8px; cursor: pointer; font-weight: 600;}
  .bulk-btn { background: #1e293b; color: white; border: 1px solid #ff3366; padding: 0.6rem 1.2rem; border-radius: 8px; cursor: pointer; margin-right: 1rem; font-weight: 600;}

  .scroll-view { flex: 1; overflow-y: auto; padding: 1rem 2rem; }

  .error { color: #f87171; }
  .empty-state { text-align: center; margin-top: 4rem; color: #64748b; }
</style>
