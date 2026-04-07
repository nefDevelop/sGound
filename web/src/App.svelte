<script>
  import { onMount } from "svelte";
  import TagEditor from "./lib/TagEditor.svelte";
  import MassiveTagEditor from "./lib/MassiveTagEditor.svelte";
  import MiniPlayer from "./lib/MiniPlayer.svelte";
  import Icon from "./lib/Icon.svelte";
  import TracksView from "./lib/TracksView.svelte";
  import AlbumsGrid from "./lib/AlbumsGrid.svelte";
  import Playlists from "./lib/Playlists.svelte";
  import PlaylistsView from "./lib/PlaylistsView.svelte";
  import Maintenance from "./lib/Maintenance.svelte";
  import SmartPlaylistView from "./lib/SmartPlaylistView.svelte";
  import Dashboard from "./lib/Dashboard.svelte";
  import Settings from "./lib/Settings.svelte";
  import Toast from "./lib/Toast.svelte";
  import FullPlayer from "./lib/FullPlayer.svelte";
  import { showToast, audioQuality } from "./lib/stores.js";
  import { get } from "svelte/store";
  import { generatePalette, applyTheme } from "./lib/theme.js";

  let currentView = $state("dashboard");
  let tracks = $state([]);
  let playlists = $state([]);
  let selectedPlaylist = $state(null);
  let selectedSmart = $state(null);
  let error = $state(null);
  let searchQuery = $state("");

  let isTagEditorOpen = $state(false);
  let isMassiveEditorOpen = $state(false);
  let trackToEdit = $state({ id: "", title: "", artist: "", album: "" });
  let selectedTracks = $state([]);

  let currentPlaying = $state(null);
  let audioRef = $state(null);
  let isFullPlayerOpen = $state(false);

  // Shared audio state
  let progress = $state(0);
  let duration = $state(0);
  let currentTime = $state(0);

  onMount(() => {
    fetchTracks();
    fetchPlaylists();
    loadTheme();
  });

  function selectSmart(config) {
    selectedSmart = config;
    currentView = "smart-playlist";
  }

  async function loadTheme() {
    try {
      const res = await fetch("/api/config");
      const config = await res.json();
      if (config.themeColor) {
        applyTheme(generatePalette(config.themeColor));
      }
    } catch (e) {
      console.error("Failed to load theme");
    }
  }

  async function fetchPlaylists() {
    try {
      const res = await fetch("/api/playlists");
      if (res.ok) playlists = await res.json();
    } catch (e) {
      console.error("Playlists fetch error");
    }
  }

  async function fetchTracks() {
    try {
      const res = await fetch("/api/tracks");
      if (!res.ok) throw new Error("Failed to fetch tracks");
      tracks = (await res.json()) || [];
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
      const res = await fetch(
        `/api/search?q=${encodeURIComponent(searchQuery)}`,
      );
      const data = await res.json();
      tracks = data.tracks || [];
    } catch (e) {
      console.error("Search failed", e);
    }
  }

  async function triggerScan() {
    try {
      const res = await fetch("/api/scan", { method: "POST" });
      if (res.ok) {
        showToast("Library scan started!", "success");
        setTimeout(fetchTracks, 2000);
      }
    } catch (e) {
      showToast("Scan failed", "error");
    }
  }

  function openEditor(track) {
    trackToEdit = { ...track };
    isTagEditorOpen = true;
  }

  function handlePlay(track) {
    currentPlaying = track;
    if (audioRef) {
      const quality = get(audioQuality);
      const url =
        `/api/stream/${track.id}` +
        (quality !== "original" ? `?bitrate=${quality}` : "");
      audioRef.src = url;
      audioRef.play();
    }
  }

  function handleSelectionChange(selected) {
    selectedTracks = selected;
  }

  async function handleDelete(track) {
    if (!confirm(`Are you sure you want to delete "${track.title}" from library?`)) return;
    try {
      const res = await fetch(`/api/tracks/${track.id}`, { method: 'DELETE' });
      if (res.ok) {
        showToast("Track deleted", "success");
        fetchTracks();
      }
    } catch (e) {
      showToast("Failed to delete", "error");
    }
  }

  function handleQueue(track) {
     showToast("Added to queue: " + track.title, "success");
     // Logic for adding to queue would go here
  }

  async function selectAlbum(albumId) {
    try {
      const res = await fetch(`/api/albums/${albumId}/tracks`);
      tracks = (await res.json()) || [];
      currentView = "library";
    } catch (e) {
      console.error(e);
    }
  }

  function selectPlaylist(playlist) {
    selectedPlaylist = playlist;
    currentView = "playlist";
  }
</script>

<main class="app-layout">
  <aside class="sidebar">
    <div class="brand">
      <div class="logo"></div>
      <h2>Sgound</h2>
    </div>

    <nav>
      <button
        class="nav-item"
        class:active={currentView === "dashboard"}
        on:click={() => (currentView = "dashboard")}
      >
        Dashboard
      </button>
      <button
        class="nav-item"
        class:active={currentView === "library"}
        on:click={() => (currentView = "library")}
      >
        Library
      </button>
      <button
        class="nav-item"
        class:active={currentView === "albums"}
        on:click={() => (currentView = "albums")}
      >
        Albums
      </button>

      <button
        class="nav-item"
        class:active={currentView === "playlists"}
        on:click={() => (currentView = "playlists")}
      >
        Playlists
      </button>

      <button
        class="nav-item"
        class:active={currentView === "settings"}
        on:click={() => (currentView = "settings")}
      >
        Settings
      </button>

      <button
        class="nav-item"
        class:active={currentView === "maintenance"}
        on:click={() => (currentView = "maintenance")}
      >
        Maintenance
      </button>

      <div class="nav-divider"></div>
      <div class="nav-label">Direct Access</div>

      {#each playlists as pl}
        <button
          class="nav-item playlist-item"
          class:active={currentView === "playlist" &&
            selectedPlaylist?.id === pl.id}
          on:click={() => {
            selectedPlaylist = pl;
            currentView = "playlist";
          }}
        >
          <Icon name="list" size={18} />
          <span>{pl.name}</span>
        </button>
      {/each}
    </nav>
  </aside>

  <section class="content">
    <header class="top-bar">
      <div class="search-box">
        <input
          type="text"
          placeholder="Search music..."
          bind:value={searchQuery}
          on:input={handleSearch}
        />
      </div>
      <div class="actions">
        {#if selectedTracks.length > 1}
          <button
            class="bulk-btn"
            on:click={() => (isMassiveEditorOpen = true)}
          >
            Edit ({selectedTracks.length})
          </button>
        {/if}
        <button class="primary-btn" on:click={triggerScan}>Scan Library</button>
      </div>
    </header>

    <div class="scroll-view">
      {#if error}
        <p class="error">{error}</p>
      {:else if currentView === "dashboard"}
        <Dashboard 
          on:play={(e) => handlePlay(e.detail)} 
          on:edit={(e) => openEditor(e.detail)}
          on:queue={(e) => handleQueue(e.detail)}
          on:delete={(e) => handleDelete(e.detail)}
        />
      {:else if currentView === "library"}
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
            on:queue={(e) => handleQueue(e.detail)}
            on:delete={(e) => handleDelete(e.detail)}
          />
        {/if}
      {:else if currentView === "albums"}
        <AlbumsGrid on:selectAlbum={(e) => selectAlbum(e.detail)} />
      {:else if currentView === "playlists"}
        <Playlists 
          on:selectPlaylist={(e) => selectPlaylist(e.detail)} 
          on:selectSmart={(e) => selectSmart(e.detail)}
        />
      {:else if currentView === "smart-playlist"}
        <SmartPlaylistView 
          {...selectedSmart}
          {currentPlaying}
          on:back={() => currentView = "playlists"}
          on:play={(e) => handlePlay(e.detail)}
          on:edit={(e) => openEditor(e.detail)}
          on:delete={(e) => handleDelete(e.detail)}
          on:queue={(e) => handleQueue(e.detail)}
        />
      {:else if currentView === "playlist"}
        <PlaylistsView
          playlist={selectedPlaylist}
          {currentPlaying}
          on:play={(e) => handlePlay(e.detail)}
          on:edit={(e) => openEditor(e.detail)}
          on:selectionChange={(e) => handleSelectionChange(e.detail)}
          on:delete={(e) => handleDelete(e.detail)}
          on:queue={(e) => handleQueue(e.detail)}
        />
      {:else if currentView === "settings"}
        <Settings />
      {:else if currentView === "maintenance"}
        <Maintenance on:fixTags={() => currentView = "library"} />
      {/if}
    </div>
  </section>

  <MiniPlayer
    {currentPlaying}
    bind:audioRef
    bind:progress
    bind:duration
    bind:currentTime
    on:openFull={() => (isFullPlayerOpen = true)}
    on:edit={(e) => openEditor(e.detail)}
    on:delete={(e) => handleDelete(e.detail)}
  />
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

<Toast />

<FullPlayer
  {currentPlaying}
  {audioRef}
  bind:isOpen={isFullPlayerOpen}
  bind:progress
  bind:duration
  bind:currentTime
  on:edit={(e) => openEditor(e.detail)}
  on:delete={(e) => handleDelete(e.detail)}
/>

<style>
  .app-layout {
    display: grid;
    grid-template-areas: "sidebar content" "player player";
    grid-template-columns: 280px 1fr;
    grid-template-rows: 1fr 110px;
    height: 100vh;
    background: var(--surface);
    color: var(--text-main);
  }

  .sidebar {
    grid-area: sidebar;
    background: var(--surface);
    padding: 2.5rem 1.2rem;
    display: flex;
    flex-direction: column;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 0 1rem;
    margin-bottom: 2.5rem;
  }

  .logo {
    width: 32px;
    height: 32px;
    background: var(--primary);
    border-radius: 10px;
    box-shadow: 0 4px 12px var(--primary-10);
  }

  .brand h2 { font-size: 1.4rem; margin: 0; font-weight: 900; letter-spacing: -0.5px; }

  nav { display: flex; flex-direction: column; gap: 0.3rem; }

  .nav-item {
    display: flex;
    align-items: center;
    gap: 1rem;
    width: 100%;
    padding: 0.8rem 1.2rem;
    background: transparent;
    border: none;
    color: var(--text-muted);
    text-align: left;
    border-radius: 14px;
    cursor: pointer;
    font-weight: 700;
    font-size: 0.95rem;
    transition: all 0.2s cubic-bezier(0.2, 0, 0, 1);
  }

  .nav-item:hover { background: var(--surface-container); color: var(--text-main); }
  .nav-item.active {
    background: var(--primary);
    color: #ffffff;
    box-shadow: 0 4px 12px var(--primary-10);
  }

  .nav-label {
    padding: 1.5rem 1.2rem 0.8rem;
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 1.5px;
    color: var(--text-muted);
    font-weight: 800;
    opacity: 0.5;
  }

  .content {
    grid-area: content;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: var(--surface-container);
    margin: 1rem 1rem 0 0;
    border-radius: var(--radius) var(--radius) 0 0;
    border: 1px solid var(--border-color);
    border-bottom: none;
  }

  .top-bar {
    padding: 1.5rem 2.5rem;
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: transparent;
  }

  .search-box input {
    background: var(--surface-container-high);
    border: 1px solid var(--border-color);
    padding: 0.8rem 1.5rem;
    border-radius: 100px;
    color: white;
    width: 350px;
    outline: none;
    font-weight: 600;
    transition: all 0.2s;
  }
  .search-box input:focus { border-color: var(--primary); background: var(--surface); width: 400px; }

  .actions { display: flex; gap: 1rem; }

  .bulk-btn {
    background: var(--surface-container-high);
    color: var(--primary);
    border: 1px solid var(--primary);
    padding: 0.6rem 1.2rem;
    border-radius: 100px;
    font-weight: 700;
  }

  .scroll-view {
    flex: 1;
    overflow-y: auto;
    padding: 1rem 2.5rem 3rem;
  }

  .error { color: #f87171; background: rgba(248, 113, 113, 0.1); padding: 1rem; border-radius: 12px; margin: 1rem; }
  .empty-state { text-align: center; margin-top: 6rem; color: var(--text-muted); }
</style>
