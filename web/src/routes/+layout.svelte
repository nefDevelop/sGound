<script>
  import { onMount, setContext } from "svelte";
  import { fade } from "svelte/transition";
  import { page } from "$app/stores";
  import { onNavigate, goto } from "$app/navigation";
  import "../app.css";
  import { 
    showToast, queue, currentIndex, isPlaying, inboxCount, rawCount, 
    currentView, viewParams, isLoggedIn, setupRequired, apiFetch,
    contextMenu, hideContextMenu
  } from "$lib/stores.js";
  import { generatePalette, applyTheme } from "$lib/theme.js";
  import { get } from "svelte/store";
  
  import Sidebar from "$lib/Sidebar.svelte";
  import ContextMenu from "$lib/ContextMenu.svelte";
  import MiniPlayer from "$lib/MiniPlayer.svelte";
  import PlayerAnchorBar from "$lib/PlayerAnchorBar.svelte";
  import FullPlayer from "$lib/FullPlayer.svelte";
  import Search from "$lib/Search.svelte";
  import Icon from "$lib/Icon.svelte";
  import QueuePanel from "$lib/QueuePanel.svelte";
  import Login from "$lib/Login.svelte";
  import Setup from "$lib/Setup.svelte";
  import TagEditor from "$lib/TagEditor.svelte";
  import MassiveTagEditor from "$lib/MassiveTagEditor.svelte";
  import Toast from "$lib/Toast.svelte";
  import ConfirmModal from "$lib/ConfirmModal.svelte";
  import AudioEngine from "$lib/AudioEngine.svelte";

  let { children } = $props();

  // State (from App.svelte)
  let setupUsername = $state('');
  let tracks = $state([]);
  let loadingTracks = $state(false);
  let tracksOffset = $state(0);
  let hasMoreTracks = $state(true);
  const tracksLimit = 500;
  let playlists = $state([]);
  let scanStatus = $state({ scanning: false, processed: 0, inboxCount: 0, rawCount: 0, isPaused: false, ramUsedMB: 0 });
  let error = $state(null);
  
  let isTagEditorOpen = $state(false);
  let isMassiveEditorOpen = $state(false);
  let trackToEdit = $state({ id: "", title: "", artist: "", album: "" });
  let selectedTracks = $state([]);
  
  let audioRef = $state(null);
  let progress = $state(0);
  let duration = $state(0);
  let currentTime = $state(0);
  let playerSubView = $state("cover");
  
  let skipNext = $state();
  let skipBack = $state();
  
  // Reactive current track
  let currentPlaying = $derived($currentIndex >= 0 && $currentIndex < $queue.length ? $queue[$currentIndex] : null);
  
  let pollingInterval = null;
  let confirmState = $state({ isOpen: false, title: "", message: "", confirmLabel: "Confirm", type: "info", icon: null, onconfirm: null, oncancel: null });
  let isQueueOpen = $state(false);

  // Dynamic Favicon
  $effect(() => {
    const favicon = document.querySelector('link[rel="icon"]');
    if (!(favicon instanceof HTMLLinkElement)) return;
    favicon.href = currentPlaying ? `/api/cover/${currentPlaying.id}` : "/favicons.svg";
  });

  // --- Core Handlers ---
  async function fetchTracks(append = false, silent = false) {
    if (loadingTracks || (!hasMoreTracks && append)) return;
    try {
      if (!silent) loadingTracks = true;
      if (!append) { tracksOffset = 0; hasMoreTracks = true; if (!silent) tracks = []; }
      const res = await apiFetch(`/api/tracks?limit=${tracksLimit}&offset=${tracksOffset}&minimal=true`);
      if (!res.ok) throw new Error("Failed to fetch tracks");
      const newTracks = (await res.json()) || [];
      tracks = append ? [...tracks, ...newTracks] : newTracks;
      hasMoreTracks = newTracks.length === tracksLimit;
      tracksOffset += newTracks.length;
      error = null;
    } catch (e) { if (!silent) error = e.message; } finally { loadingTracks = false; }
  }

  async function checkScanStatus() {
    if (!$isLoggedIn) return;
    try {
      const res = await apiFetch("/api/library/status");
      if (!res.ok) return;
      const data = await res.json();
      const wasScanning = scanStatus.scanning;
      scanStatus = data;
      
      inboxCount.set(data.inboxCount || 0);
      rawCount.set(data.rawCount || 0);

      if (data.scanning && !pollingInterval) startPolling();
      if (wasScanning && !data.scanning) {
        stopPolling();
        fetchTracks();
        fetchPlaylists();
        showToast("Escaneo finalizado", "success");
      }
    } catch (e) { console.error(e); }
  }

  function startPolling() { if (!pollingInterval) pollingInterval = setInterval(checkScanStatus, 3000); }
  function stopPolling() { clearInterval(pollingInterval); pollingInterval = null; }

  async function loadTheme() {
    const res = await apiFetch("/api/config").catch(() => null);
    const config = res?.ok ? await res.json() : { themeColor: '#ff3366' };
    applyTheme(generatePalette(config.themeColor || '#ff3366'));
  }

  async function fetchPlaylists() {
    const res = await apiFetch("/api/playlists");
    if (res.ok) playlists = await res.json();
  }

  const handlers = {
    handleLogout: () => {
      localStorage.removeItem('username');
      isLoggedIn.set(false);
      window.location.reload();
    },
    handlePlay: (data) => {
      const { track, tracks: trackList } = data.track ? data : { track: data, tracks: [data] };
      queue.set(trackList);
      currentIndex.set(trackList.findIndex(t => t.id === track.id) || 0);
      isPlaying.set(true);
    },
    handleShufflePlay: (trackList) => {
      if (!trackList?.length) return;
      queue.set([...trackList].sort(() => Math.random() - 0.5));
      currentIndex.set(0);
      isPlaying.set(true);
    },
    openEditor: async (item) => {
      try {
        const res = await apiFetch(`/api/tracks?id=${item.id}&limit=1`);
        if (res.ok) {
          const fullData = await res.json();
          trackToEdit = fullData?.[0] || item;
        } else {
          trackToEdit = item;
        }
      } catch (e) {
        trackToEdit = item;
      }
      isTagEditorOpen = true;
    },
    openAlbumEditor: async (album) => {
      loadingTracks = true;
      try {
        const res = await apiFetch(`/api/albums/${album.id}/tracks`);
        if (res.ok) {
          selectedTracks = await res.json();
          isMassiveEditorOpen = true;
        } else {
          showToast("Error al cargar pistas del álbum", "error");
        }
      } catch (e) {
        showToast("Error de conexión", "error");
      } finally {
        loadingTracks = false;
      }
    },
    handleSelectionChange: (s) => (selectedTracks = s),
    triggerScan: async () => {
      const res = await apiFetch("/api/scan", { method: "POST" });
      if (res.ok) { showToast("Scan started", "success"); checkScanStatus(); }
    },
    handleDeletePhysical: async (track, callback) => {
      confirmState = { 
        isOpen: true, title: "Delete File", message: `Permanently delete ${track.title}?`, 
        type: "danger", confirmLabel: "Delete", icon: 'trash',
        onconfirm: async () => {
          const res = await apiFetch(`/api/tracks/${track.id}`, { method: 'DELETE' });
          if (res.ok) { showToast("Deleted", "success"); fetchTracks(); callback?.(); }
        },
        oncancel: () => { confirmState.isOpen = false; }
      };
    },
    handleReject: async (track, callback) => {
      const res = await apiFetch(`/api/tracks/reject`, {
        method: 'PUT', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ids: [track.id] })
      });
      if (res.ok) { showToast("Hidden", "success"); fetchTracks(); callback?.(); }
    },
    handleBulkReject: async (ids, callback, refresh = true) => {
      const res = await apiFetch(`/api/tracks/reject`, {
        method: 'PUT', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ids })
      });
      if (res.ok && refresh) { fetchTracks(); callback?.(); }
      return res.ok;
    },
    handlePlayAlbum: async (albumId) => {
      try {
        const res = await apiFetch(`/api/albums/${albumId}/tracks`);
        if (res.ok) {
          const tracks = await res.json();
          if (tracks.length > 0) handlers.handlePlay({ track: tracks[0], tracks });
        }
      } catch (e) { showToast("Error al reproducir álbum", "error"); }
    },
    handleQueueAlbum: async (albumId) => {
      try {
        const res = await apiFetch(`/api/albums/${albumId}/tracks`);
        if (res.ok) {
          const tracks = await res.json();
          if (tracks.length > 0) {
            queue.update(q => [...q, ...tracks]);
            showToast(`Añadidas ${tracks.length} canciones a la cola`, "success");
          }
        }
      } catch (e) { showToast("Error al añadir álbum a la cola", "error"); }
    },
    handleQueue: (track) => {
      queue.update(q => [...q, track]);
      showToast(`Añadida "${track.title}" a la cola`, "success");
    },
    handleSkipNext: () => {
      const q = get(queue);
      if (q.length === 0) return;
      currentIndex.update(n => (n + 1) % q.length);
      isPlaying.set(true);
    },
    handleSkipBack: () => {
      const q = get(queue);
      if (q.length === 0) return;
      currentIndex.update(n => (n - 1 + q.length) % q.length);
      isPlaying.set(true);
    },
    handleSearchResultSelect: (type, item) => {
      if (type === 'album') goto(`/album/${item.id}`);
      else if (type === 'artist') goto(`/artist/${item.id}`);
      else if (type === 'track') handlers.handlePlay(item);
    },
    handleSearchResultPlay: (item) => handlers.handlePlay(item),
    handleSearch: () => {},
    handleFixTags: (track) => handlers.openEditor(track),
    fetchTracks,
    checkScanStatus
  };

  // Provide state and handlers to all routes
  setContext("app", {
    get handlers() { return handlers; },
    get scanStatus() { return scanStatus; },
    get tracks() { return tracks; },
    get loadingTracks() { return loadingTracks; },
    get currentPlaying() { return currentPlaying; }
  });

  handlers.handleSkipNext = () => skipNext?.();
  handlers.handleSkipBack = () => skipBack?.();

  async function checkAuthStatus() {
    try {
      const res = await apiFetch("/api/auth/status");
      if (res.status === 401) {
        isLoggedIn.set(false);
        localStorage.removeItem('username');
      }
    } catch (e) { console.error(e); }
  }

  let statusInterval;
  onMount(() => {
    checkAuthStatus();
    loadTheme();
    if (get(isLoggedIn)) { 
      fetchTracks(); 
      fetchPlaylists(); 
      checkScanStatus(); 
    }
    
    const updateInterval = () => {
      if (statusInterval) clearInterval(statusInterval);
      const delay = scanStatus?.scanning ? 1000 : 5000;
      statusInterval = setInterval(checkScanStatus, delay);
    };

    statusInterval = setInterval(checkScanStatus, 5000);

    $effect(() => {
      if (scanStatus?.scanning) updateInterval();
      else if (statusInterval && !scanStatus?.scanning) updateInterval();
    });

    const handlePopState = () => {
      if (history.state?.view !== 'fullplayer') {
        syncViewWithRoute();
      }
    };
    window.addEventListener("popstate", handlePopState);

    return () => {
      clearInterval(statusInterval);
      window.removeEventListener("popstate", handlePopState);
    };
  });

  // --- View Transitions ---
  onNavigate((navigation) => {
    if (!document.startViewTransition) return;

    return new Promise((resolve) => {
      document.startViewTransition(async () => {
        resolve();
        await navigation.complete;
      });
    });
  });

  // Route mapping for SvelteKit compatibility
  const viewMap = {
    '/': 'dashboard',
    '/library': 'library',
    '/inbox': 'inbox',
    '/albums': 'albums',
    '/artists': 'artists',
    '/settings': 'settings',
    '/maintenance': 'maintenance'
  };

  function syncViewWithRoute() {
    const path = $page.url.pathname;
    if (viewMap[path]) {
      currentView.set(viewMap[path]);
    } else if (path.startsWith('/album/')) {
      currentView.set('album');
    } else if (path.startsWith('/artist/')) {
      currentView.set('artist-detail');
    }
  }

  $effect(() => {
    syncViewWithRoute();
  });

  let isFullPlayerActive = $derived($currentView === 'fullplayer');
</script>

<svelte:head>
  <title>{$isLoggedIn ? `${$currentView} | Sgound` : "Login | Sgound"}</title>
</svelte:head>

{#if $setupRequired}
  <Setup 
    username={setupUsername}
    on:complete={() => { setupRequired.set(false); isLoggedIn.set(true); fetchTracks(); }} 
  />
{:else if !$isLoggedIn}
  <Login 
    on:login={() => { isLoggedIn.set(true); loadTheme(); fetchTracks(); fetchPlaylists(); }} 
    on:setup={(e) => { setupUsername = e.detail.username; setupRequired.set(true); }}
  />
{:else}
  <AudioEngine 
    bind:audioRef 
    bind:progress 
    bind:duration 
    bind:currentTime 
    bind:onSkipNext={skipNext}
    bind:onSkipBack={skipBack}
  />
  <main class="app-layout" class:is-cinema={isFullPlayerActive && playerSubView === 'cinema'}>
    <Sidebar {playlists} onLogout={handlers.handleLogout} />

    <section class="content" class:full-player-view={isFullPlayerActive}>
      {#if scanStatus.isPaused}
        <div class="ram-warning-banner">
          <Icon name="alertCircle" size={16} />
          <span><b>Memory usage critical ({scanStatus.ramUsedMB}MB).</b> Scanner paused.</span>
        </div>
      {/if}
      
      {#if !isFullPlayerActive}
        <header class="top-bar">
          <div class="search-box">
            <Search 
              onselect={handlers.handleSearchResultSelect} 
              onplay={handlers.handleSearchResultPlay}
              onsearch={handlers.handleSearch}
            />
          </div>
          <div class="actions">
            {#if selectedTracks.length > 1}
              <button class="bulk-btn" onclick={() => (isMassiveEditorOpen = true)}>
                Edit ({selectedTracks.length})
              </button>
            {/if}
          </div>
        </header>
      {/if}

      <div class="scroll-view" class:is-fullplayer={isFullPlayerActive}>
        {#if error}
          <div class="error-state">
            <Icon name="alertCircle" size={48} color="var(--primary)" />
            <p class="error-msg">{error}</p>
            <button class="retry-btn" onclick={() => handlers.fetchTracks()}>
              <Icon name="refreshCw" size={16} /> Reintentar carga
            </button>
          </div>
        {:else if isFullPlayerActive && currentPlaying}
          <FullPlayer 
            {currentPlaying}
            {audioRef}
            isOpen={true}
            bind:progress
            bind:duration
            bind:currentTime
            bind:subView={playerSubView}
            onedit={handlers.openEditor}
            onhide={handlers.handleReject}
            ondeletePhysical={handlers.handleDeletePhysical}
            onskipNext={handlers.handleSkipNext}
            onskipBack={handlers.handleSkipBack}
          />
        {:else if isFullPlayerActive && !currentPlaying}
          <div class="empty-player-state">
            <div style="opacity: 0.2">
              <Icon name="music" size={64} />
            </div>
            <p>No hay ninguna pista en reproducción</p>
            <button class="subtle-btn" onclick={() => { syncViewWithRoute(); history.back(); }}>Volver</button>
          </div>
        {:else}
          {@render children()}
        {/if}
      </div>
    </section>

    <div class="player-area" class:is-fullplayer={isFullPlayerActive}>
      <div class="player-stack">
        <div class:hidden={!isFullPlayerActive} in:fade={{duration: 150, delay: 100}} out:fade={{duration: 150}}>
          <PlayerAnchorBar
            {currentPlaying}
            bind:view={playerSubView}
            oncloseFull={() => {
              syncViewWithRoute();
              history.back();
            }}
          />
        </div>
        <div class:hidden={isFullPlayerActive} in:fade={{duration: 150, delay: 100}} out:fade={{duration: 150}}>
          <MiniPlayer
            {currentPlaying}
            bind:audioRef
            bind:progress
            bind:duration
            bind:currentTime
            onopenFull={() => {
              currentView.set("fullplayer");
              history.pushState({ view: "fullplayer", params: {} }, "");
            }}
            onedit={handlers.openEditor}
            onhide={handlers.handleReject}
            ondeletePhysical={handlers.handleDeletePhysical}
            ontoggleQueue={() => isQueueOpen = !isQueueOpen}
          />
        </div>
      </div>
    </div>
  </main>

  <QueuePanel bind:isOpen={isQueueOpen} />
  <TagEditor bind:isOpen={isTagEditorOpen} bind:track={trackToEdit} onclose={() => fetchTracks(false)} />
  <MassiveTagEditor bind:isOpen={isMassiveEditorOpen} {selectedTracks} onclose={() => fetchTracks(false)} />
  <Toast />
  <ConfirmModal bind:isOpen={confirmState.isOpen} {...confirmState} onconfirm={confirmState.onconfirm} />
  
  <ContextMenu 
    bind:visible={$contextMenu.visible}
    x={$contextMenu.x}
    y={$contextMenu.y}
    items={$contextMenu.items}
    onclose={hideContextMenu}
  />
{/if}

<style>
  :global(:root) {
    --primary: #ff3366;
    --primary-rgb: 255, 51, 102;
    --surface: #000000;
    --surface-container: #121212;
    --surface-container-highest: #1a1a1a;
    --border-color: rgba(255,255,255,0.1);
    --text-main: #ffffff;
    --text-muted: #a0a0a0;
  }

  :global(body) {
    margin: 0;
    padding: 0;
    font-family: 'Outfit', sans-serif;
    background: var(--surface);
    color: var(--text-main);
  }

  .app-layout {
    display: grid;
    grid-template-areas: 
      "sidebar content"
      "sidebar player";
    grid-template-columns: 280px minmax(0, 1fr);
    grid-template-rows: 1fr auto;
    height: 100vh;
    background: var(--surface);
    color: var(--text-main);
    overflow: hidden;
    width: 100%;
    isolation: isolate;
  }

  .content {
    grid-area: content;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    min-width: 0;
    background: var(--surface-container);
    margin: 1rem 1rem 0.5rem 1rem;
    border-radius: 28px;
    border: 1px solid var(--border-color);
    box-shadow: 0 8px 32px rgba(0,0,0,0.12);
    transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .content.full-player-view {
    margin-bottom: 0;
    border-bottom-left-radius: 0;
    border-bottom-right-radius: 0;
    border-bottom: none;
    box-shadow: none;
  }

  .scroll-view {
    flex: 1;
    overflow-y: auto;
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
    scrollbar-width: none;
    -ms-overflow-style: none;
  }
  .scroll-view.is-fullplayer { padding: 0; }
  .scroll-view::-webkit-scrollbar { display: none; }

  .player-area {
    grid-area: player;
    margin: 0.5rem 1rem 1rem 1rem;
    border-radius: 28px;
    display: flex;
    flex-direction: column;
    background: var(--surface-container-highest);
    border: 1px solid var(--border-color);
    z-index: 1; /* Explicitly low z-index */
    transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .player-area.is-fullplayer {
    margin-top: 0;
    border-top-left-radius: 0;
    border-top-right-radius: 0;
    border-top: none;
    background: var(--surface-container-highest);
  }

  .player-stack {
    display: grid;
    grid-template-columns: 1fr;
    grid-template-rows: 1fr;
    height: 100px;
  }

  .player-stack > div {
    grid-column: 1;
    grid-row: 1;
    height: 100px;
  }

  .top-bar {
    height: 70px;
    display: flex;
    align-items: center;
    padding: 0 2rem;
    gap: 2rem;
    background: var(--surface-05);
    backdrop-filter: blur(20px);
    border-bottom: 1px solid var(--white-05);
  }

  .search-box { 
    flex: 1;
    max-width: 400px; 
  }
  
  .ram-warning-banner {
    background: var(--primary);
    color: white;
    padding: 0.6rem 1.5rem;
    font-size: 0.85rem;
    display: flex;
    align-items: center;
    gap: 0.8rem;
  }

  .bulk-btn {
    background: var(--surface-container-high);
    color: var(--primary);
    border: 1px solid var(--primary-30);
    padding: 0.6rem 1.2rem;
    border-radius: 100px;
    font-weight: 700;
    cursor: pointer;
  }

  .error-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1.5rem;
    padding: 4rem 2rem;
    text-align: center;
    background: rgba(248, 113, 113, 0.05);
    border: 1px dashed rgba(248, 113, 113, 0.2);
    border-radius: 24px;
    margin: 2rem;
  }
  
  .empty-player-state {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1.5rem;
    color: var(--text-muted);
    text-align: center;
    padding: 2rem;
  }

  .empty-player-state p {
    font-size: 1.1rem;
    font-weight: 500;
    max-width: 250px;
    line-height: 1.4;
  }

  .subtle-btn {
    background: var(--surface-container-high);
    color: var(--text-main);
    border: 1px solid var(--border-color);
    padding: 0.6rem 1.4rem;
    border-radius: 12px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
  }
  .subtle-btn:hover {
    background: var(--surface-container-highest);
    border-color: var(--text-muted);
  }

  .retry-btn {
    background: var(--primary);
    color: white;
    border: none;
    padding: 0.8rem 1.8rem;
    border-radius: 14px;
    font-weight: 800;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 0.6rem;
    box-shadow: 0 4px 15px var(--primary-20);
  }

  @media (max-width: 1100px) {
    .app-layout { grid-template-columns: 80px minmax(0, 1fr); }
  }

  @media (max-width: 768px) {
    .app-layout {
      grid-template-areas: "content" "player" "sidebar";
      grid-template-columns: minmax(0, 1fr);
      grid-template-rows: 1fr auto auto;
    }
    .content {
      margin: 0.5rem 0.5rem 0 0.5rem;
    }
    .player-area {
      margin: 0.5rem;
      border-radius: 20px;
    }
    .player-stack {
      height: 72px;
    }
    .top-bar {
      padding: 0 1rem;
      gap: 1rem;
    }
  }

  .hidden {
    display: none !important;
  }
</style>
