<script>
  import { onMount } from "svelte";
  import { get } from "svelte/store";
  import { 
    showToast, queue, currentIndex, isPlaying, inboxCount, rawCount, 
    currentView, viewParams, isLoggedIn, setupRequired, apiFetch
  } from "./lib/stores.js";
  import { generatePalette, applyTheme } from "./lib/theme.js";
  
  import AppShell from "./lib/AppShell.svelte";
  import Login from "./lib/Login.svelte";
  import Setup from "./lib/Setup.svelte";
  import TagEditor from "./lib/TagEditor.svelte";
  import MassiveTagEditor from "./lib/MassiveTagEditor.svelte";
  import Toast from "./lib/Toast.svelte";
  import ConfirmModal from "./lib/ConfirmModal.svelte";

  // State
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
  
  let currentPlaying = $state(null);
  let pollingInterval = null;
  let confirmState = $state({ isOpen: false, title: "", message: "", confirmLabel: "Confirm", type: "info", icon: null, onconfirm: null, oncancel: null });

  // Sync currentPlaying
  $effect(() => {
    const q = $queue;
    const idx = $currentIndex;
    if (idx >= 0 && idx < q.length) currentPlaying = q[idx];
  });

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
      
      // Update global stores for real-time UI updates (badge, dots)
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

  // --- Action Handlers ---

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
    handleTracksScroll: (e) => {
      if (e.target.scrollHeight - e.target.scrollTop - e.target.clientHeight < 500) {
        if ($currentView === 'library') fetchTracks(true);
      }
    },
    openEditor: async (item) => {
      // Fetch full track data (not minimal) before opening editor
      try {
        const res = await apiFetch(`/api/tracks?id=${item.id}&limit=1`);
        if (res.ok) {
          const fullData = await res.json();
          if (fullData && fullData.length > 0) {
            trackToEdit = { ...fullData[0] };
          } else {
            trackToEdit = { ...item };
          }
        } else {
          trackToEdit = { ...item };
        }
      } catch (e) {
        trackToEdit = { ...item };
      }
      isTagEditorOpen = true;
    },
    handleSelectionChange: (s) => (selectedTracks = s),
    handleSearch: async (query) => {
      if (!query) { fetchTracks(); return; }
      const res = await apiFetch(`/api/search?q=${encodeURIComponent(query)}`);
      if (res.ok) { tracks = (await res.json()).tracks || []; currentView.set("library"); }
    },
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
    fetchTracks,
    checkScanStatus
  };

  async function checkAuthStatus() {
    try {
      const res = await apiFetch("/api/auth/status");
      if (res.status === 401) {
        isLoggedIn.set(false);
        localStorage.removeItem('username');
      }
    } catch (e) { console.error(e); }
  }

  onMount(() => {
    checkAuthStatus();
    loadTheme();
    if (get(isLoggedIn)) { fetchTracks(); fetchPlaylists(); checkScanStatus(); }
    setInterval(checkScanStatus, 5000);

    window.addEventListener('popstate', (e) => {
      if (e.state?.view) { currentView.set(e.state.view); viewParams.set(e.state.params || {}); }
    });
  });

  let pageTitle = $derived($isLoggedIn ? `${$currentView} | Sgound` : "Login | Sgound");
</script>

<svelte:head>
  <title>{pageTitle}</title>
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
  <AppShell 
    isLoggedIn={$isLoggedIn}
    {playlists}
    {scanStatus}
    {tracks}
    {loadingTracks}
    {currentPlaying}
    bind:audioRef
    bind:progress
    bind:duration
    bind:currentTime
    bind:playerSubView
    {error}
    bind:isMassiveEditorOpen
    {selectedTracks}
    {handlers}
  />
{/if}

<TagEditor bind:isOpen={isTagEditorOpen} bind:track={trackToEdit} onclose={() => fetchTracks(false)} />
<MassiveTagEditor bind:isOpen={isMassiveEditorOpen} {selectedTracks} onclose={() => fetchTracks(false)} />
<Toast />
<ConfirmModal bind:isOpen={confirmState.isOpen} {...confirmState} onconfirm={confirmState.onconfirm} />
