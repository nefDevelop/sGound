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
        <Dashboard on:play={(e) => handlePlay(e.detail)} />
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
          />
        {/if}
      {:else if currentView === "albums"}
        <AlbumsGrid on:selectAlbum={(e) => selectAlbum(e.detail)} />
      {:else if currentView === "playlists"}
        <Playlists on:selectPlaylist={(e) => selectPlaylist(e.detail)} />
      {:else if currentView === "playlist"}
        <PlaylistsView
          playlist={selectedPlaylist}
          on:play={(e) => handlePlay(e.detail)}
        />
      {:else if currentView === "settings"}
        <Settings />
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
    border-right: 1px solid rgba(255, 255, 255, 0.05);
  }

  .logo {
    width: 32px;
    height: 32px;
    background: #ff3366;
    border-radius: 8px;
    margin-bottom: 2rem;
  }

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

  .nav-header {
    font-size: 0.7rem;
    text-transform: uppercase;
    color: #64748b;
    letter-spacing: 1.5px;
    font-weight: 800;
    margin: 1.5rem 0 0.5rem 1rem;
    opacity: 0.6;
  }

  .nav-divider {
    height: 1px;
    background: rgba(255, 255, 255, 0.05);
    margin: 1rem 0;
  }

  .playlist-item {
    display: flex !important;
    align-items: center;
    gap: 0.8rem;
  }

  .create-pl {
    border: 1px dashed rgba(255, 255, 255, 0.1) !important;
    margin-top: 1rem;
    color: #64748b !important;
    display: flex !important;
    align-items: center;
    gap: 0.8rem;
  }
  .create-pl:hover {
    border-color: #ff3366 !important;
    color: #ff3366 !important;
  }

  .nav-item.active {
    background: var(--primary-10);
    color: var(--primary);
  }

  .nav-divider {
    height: 1px;
    background: rgba(255, 255, 255, 0.05);
    margin: 1.5rem 0.5rem;
  }

  .nav-label {
    padding: 0 1rem;
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 1.5px;
    color: #475569;
    font-weight: 800;
    margin-bottom: 0.8rem;
  }

  .content {
    grid-area: content;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

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

  .primary-btn {
    background: var(--primary);
    color: white;
    border: none;
    padding: 0.6rem 1.2rem;
    border-radius: 8px;
    cursor: pointer;
    font-weight: 600;
  }
  .bulk-btn {
    background: #1e293b;
    color: white;
    border: 1px solid var(--primary);
    padding: 0.6rem 1.2rem;
    border-radius: 8px;
    cursor: pointer;
    margin-right: 1rem;
    font-weight: 600;
  }

  .scroll-view {
    flex: 1;
    overflow-y: auto;
    padding: 1rem 2rem;
  }

  .error {
    color: #f87171;
  }
  .empty-state {
    text-align: center;
    margin-top: 4rem;
    color: #64748b;
  }
</style>
