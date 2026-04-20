<script>
  import { onMount } from "svelte";
  import TagEditor from "./lib/TagEditor.svelte";
  import MassiveTagEditor from "./lib/MassiveTagEditor.svelte";
  import MiniPlayer from "./lib/MiniPlayer.svelte";
  import PlayerAnchorBar from "./lib/PlayerAnchorBar.svelte";
  import Icon from "./lib/Icon.svelte";
  import TracksView from "./lib/TracksView.svelte";
  import AlbumsGrid from "./lib/AlbumsGrid.svelte";
  import ArtistsView from "./lib/ArtistsView.svelte";
  import ArtistDetail from "./lib/ArtistDetail.svelte";
  import AlbumView from "./lib/AlbumView.svelte";
  import Playlists from "./lib/Playlists.svelte";
  import PlaylistsView from "./lib/PlaylistsView.svelte";
  import Maintenance from "./lib/Maintenance.svelte";
  import SmartPlaylistView from "./lib/SmartPlaylistView.svelte";
  import Dashboard from "./lib/Dashboard.svelte";
  import Settings from "./lib/Settings.svelte";
  import InboxView from "./lib/InboxView.svelte";
  import Toast from "./lib/Toast.svelte";
  import FullPlayer from "./lib/FullPlayer.svelte";
  import { showToast, audioQuality, queue, currentIndex, isPlaying, inboxCount, rawCount } from "./lib/stores.js";
  import { get } from "svelte/store";
  import { generatePalette, applyTheme } from "./lib/theme.js";
  import Login from "./lib/Login.svelte";
  import Setup from "./lib/Setup.svelte";
  import Search from "./lib/Search.svelte";
  import ConfirmModal from "./lib/ConfirmModal.svelte";
  import QueuePanel from "./lib/QueuePanel.svelte";

  // Global fetch interceptor
  const originalFetch = window.fetch;
  window.fetch = (url, options = {}) => {
    const apiKey = localStorage.getItem('apiKey');
    const urlString = typeof url === 'string' ? url : (url instanceof Request ? url.url : url.toString());

    if (urlString.startsWith('/api/') && apiKey) {
      options.headers = { ...options.headers, 'X-API-Key': apiKey };
    }
    
    return originalFetch(url, options).then(response => {
      // Si recibimos un 401 (No autorizado) y "creemos" que estamos dentro, forzamos logout
      if (response.status === 401 && !urlString.includes('/api/auth/login')) {
         if (isLoggedIn) {
           console.warn("Sesión caducada o inválida. Redirigiendo a login...");
           handleLogout();
         }
      }
      return response;
    });
  };

  let setupRequired = $state(false);
  let setupUsername = $state('');
  let isLoggedIn = $state(!!localStorage.getItem('username'));
  let currentView = $state("dashboard");
  let viewParams = $state({});
  let isNavigatingHistory = false;
  let tracks = $state([]);
  let loadingTracks = $state(false);
  let tracksOffset = $state(0);
  let hasMoreTracks = $state(true);
  const tracksLimit = 500;
  let playlists = $state([]);
  let selectedPlaylist = $state(null);
  let selectedSmart = $state(null);
  let error = $state(null);

  let isTagEditorOpen = $state(false);
  let isMassiveEditorOpen = $state(false);
  let trackToEdit = $state({ id: "", title: "", artist: "", album: "" });
  let selectedTracks = $state([]);
  let selectedArtistId = $state(null);
  let selectedAlbumId = $state(null);
  let selectedMaintenanceView = $state('overview');
  let playerSubView = $state("cover"); // "cover", "lyrics", "queue"

  let currentPlaying = $state(null);
  let audioRef = $state(null);
  let isFullPlayerOpen = $state(false);
  let isQueueOpen = $state(false);
  let maintenanceRefresh = $state();

  // Reset scroll on navigation
  $effect(() => {
    if (currentView) {
      const scrollView = document.querySelector('.scroll-view');
      if (scrollView) scrollView.scrollTop = 0;
    }
  });

  // Sync currentPlaying with queue
  $effect(() => {
    const q = $queue;
    const idx = $currentIndex;
    if (idx >= 0 && idx < q.length) {
      currentPlaying = q[idx];
    }
  });

  // Shared audio state
  let progress = $state(0);
  let duration = $state(0);
  let currentTime = $state(0);

  // Dynamic Favicon Effect
  $effect(() => {
    const favicon = document.querySelector('link[rel="icon"]');
    if (!(favicon instanceof HTMLLinkElement)) return;
    
    if (currentPlaying) {
      favicon.href = `/api/cover/${currentPlaying.id}`;
      favicon.type = "image/jpeg";
    } else {
      favicon.href = "/favicons.svg";
      favicon.type = "image/svg+xml";
    }
  });

  let scanStatus = $state({ scanning: false, processed: 0, inboxCount: 0, rawCount: 0 });
  let pollingInterval = null;
  let dashRef = $state(null);
  let inboxRef = $state(null);
  let smartRef = $state(null);
  let playlistRef = $state(null);

  let handleSkipNext = $state();
  let handleSkipBack = $state();
  let confirmState = $state({ isOpen: false, title: "", message: "", confirmLabel: "Confirm", type: "info", icon: null, onconfirm: null, oncancel: null });

  function openConfirm(options) {
    confirmState = {
      isOpen: true,
      title: options.title || "Confirmar",
      message: options.message || "¿Estás seguro?",
      confirmLabel: options.confirmLabel || "Confirmar",
      type: options.type || "info",
      icon: options.icon || null,
      onconfirm: options.onconfirm,
      oncancel: options.oncancel || null
    };
  }

  async function checkScanStatus() {
    if (!isLoggedIn) return;
    try {
      const res = await fetch("/api/library/status");
      if (!res.ok) return;
      const data = await res.json();
      
      const wasScanning = scanStatus.scanning;
      const prevInbox = $inboxCount;
      const prevRaw = $rawCount;
      
      scanStatus = data;
      inboxCount.set(data.inboxCount);
      rawCount.set(data.rawCount || 0);

      // REGLA: Solo refrescar si ha cambiado el INBOX o el estado RAW o si el escaneo ha terminado/empezado
      if (data.inboxCount !== prevInbox || data.rawCount !== prevRaw || wasScanning !== data.scanning) {
         // Si estamos escaneando, NO refrescamos la biblioteca (solo el Inbox o Dashboard)
         // Esto evita saturar el HDD con queries pesadas mientras lee archivos
         if (currentView === 'library' || currentView === 'tracks') {
            if (!data.scanning || wasScanning !== data.scanning) fetchTracks(false, true);
         }
         if (currentView === 'inbox') inboxRef?.fetchInbox(true);
         if (currentView === 'dashboard') dashRef?.refresh();
      }

      // Si el escaneo ha terminado ahora mismo
      if (wasScanning && !data.scanning) {
        stopPolling();
        fetchTracks();
        fetchPlaylists();
        inboxRef?.fetchInbox(false, false, true);
        dashRef?.refresh();
        showToast("Escaneo finalizado: <b>" + data.processed + "</b> archivos procesados", "success");
      }

      // Si detectamos que hay un escaneo en marcha y no tenemos polling, activarlo
      if (data.scanning && !pollingInterval) {
        startPolling();
      }
    } catch (e) {
      console.error("Error checking scan status", e);
    }
  }

  function startPolling() {
    if (pollingInterval) return;
    // Polling más relajado (cada 3 segundos) durante el escaneo
    pollingInterval = setInterval(checkScanStatus, 3000);
  }

  function stopPolling() {
    if (pollingInterval) {
      clearInterval(pollingInterval);
      pollingInterval = null;
    }
  }

  onMount(async () => {
    // 1. Cargar el tema inmediatamente para evitar saltos visuales
    loadTheme();

    // Escuchar cambios en otras pestañas para sincronizar el logout
    window.addEventListener('storage', (e) => {
      if (e.key === 'username' && !e.newValue) {
        isLoggedIn = false;
      }
    });

    try {
      // 2. Verificar estado global
      const resStatus = await fetch("/api/auth/status");
      
      if (isLoggedIn) {
        // 3. Cargar datos si hay sesión
        fetchTracks();
        fetchPlaylists();
        checkScanStatus();
      }

      // 4. Mantener un chequeo periódico lento (15s) para detectar escaneos de fondo
      setInterval(checkScanStatus, 15000);

    } catch (e) {
      console.error("Initialization check failed:", e);
    }
  });

  /**
   * FUNCIÓN MAESTRA DE NAVEGACIÓN
   * Centraliza los cambios de vista y el registro en el historial.
   */
  function navigate(view, params = {}, { push = true, replace = false } = {}) {
    isNavigatingHistory = true;
    
    // 1. Limpieza total de parámetros antiguos antes de asignar los nuevos
    selectedArtistId = null;
    selectedAlbumId = null;
    selectedPlaylist = null;
    selectedSmart = null;

    // 2. Asignación de la nueva vista y sus parámetros
    currentView = view;
    if (params.artistId) selectedArtistId = params.artistId;
    if (params.albumId) selectedAlbumId = params.albumId;
    if (params.playlist) selectedPlaylist = params.playlist;
    if (params.smart) selectedSmart = params.smart;
    if (params.subview) selectedMaintenanceView = params.subview;
    else if (view === 'maintenance') selectedMaintenanceView = 'overview';

    // 3. Gestión del Historial del Navegador
    if (push || replace) {
      const state = JSON.parse(JSON.stringify({ view, params }));
      if (replace) {
        history.replaceState(state, "");
      } else {
        // Solo hacemos push si el nuevo estado es diferente al actual del tope
        const currentState = history.state;
        if (!currentState || currentState.view !== view || JSON.stringify(currentState.params) !== JSON.stringify(params)) {
          history.pushState(state, "");
        }
      }
    }

    // 4. Acciones automáticas por vista
    if (view === 'library') fetchTracks();

    // Devolvemos el control a la navegación normal después de un tick
    setTimeout(() => isNavigatingHistory = false, 0);
  }

  // Sincronizar historial en el montaje
  onMount(() => {
    window.addEventListener('popstate', (event) => {
      if (event.state && event.state.view) {
        navigate(event.state.view, event.state.params || {}, { push: false });
      }
    });

    if (!history.state) {
      navigate(currentView, {
        artistId: selectedArtistId,
        albumId: selectedAlbumId,
        playlist: selectedPlaylist,
        smart: selectedSmart
      }, { replace: true });
    }
  });

  function selectSmart(config) {
    navigate("smart-playlist", { smart: config });
  }

  async function loadTheme() {
    try {
      const res = await fetch("/api/config");
      if (!res.ok) return applyTheme(generatePalette('#ff3366'));
      const config = await res.json();
      const seed = config.themeColor || '#ff3366';
      applyTheme(generatePalette(seed));
    } catch (e) {
      console.error("Failed to load theme, using fallback");
      applyTheme(generatePalette('#ff3366'));
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

  export async function fetchTracks(append = false, silent = false) {
    if (loadingTracks || (!hasMoreTracks && append)) return;
    
    try {
      if (!silent) loadingTracks = true;
      if (!append) {
        tracksOffset = 0;
        hasMoreTracks = true;
        // Solo limpiamos los tracks si NO estamos añadiendo y no es silencioso
        if (!silent && tracks.length > 0) tracks = [];
      }

      const res = await fetch(`/api/tracks?limit=${tracksLimit}&offset=${tracksOffset}&minimal=true`);
      if (!res.ok) throw new Error("Failed to fetch tracks");
      
      const newTracks = (await res.json()) || [];
      
      if (append) {
        tracks = [...tracks, ...newTracks];
      } else {
        tracks = newTracks;
      }

      hasMoreTracks = newTracks.length === tracksLimit;
      tracksOffset += newTracks.length;
      error = null;
    } catch (e) {
      if (!silent) error = e.message;
    } finally {
      loadingTracks = false;
    }
  }

  // Detectar cuando el usuario llega al final de la lista para cargar más
  function handleTracksScroll(e) {
    const target = e.target;
    // Si quedan menos de 500px para el final, cargar más
    if (target.scrollHeight - target.scrollTop - target.clientHeight < 500) {
      if (currentView === 'library') fetchTracks(true);
    }
  }

  async function handleSearch(query) {
    if (!query) {
      fetchTracks();
      return;
    }
    try {
      const res = await fetch(
        `/api/search?q=${encodeURIComponent(query)}`,
      );
      if (!res.ok) throw new Error("Search failed");
      const data = await res.json();
      tracks = data.tracks || [];
      navigate("library");
    } catch (e) {
      console.error("Search failed", e);
    }
  }

  function handleSearchResultSelect(type, item) {
    if (type === 'track') {
       handlePlay(item);
    } else if (type === 'album') {
       selectAlbum(item.id);
    } else if (type === 'artist') {
       handleSelectArtist(item);
    } else if (type === 'genre') {
       // Filter tracks by genre
       fetchTracksByGenre(item);
    } else if (type === 'playlist') {
       selectPlaylist(item);
    }
  }

  async function fetchTracksByGenre(genre) {
    try {
      const res = await fetch(`/api/tracks?genre=${encodeURIComponent(genre)}`);
      if (!res.ok) throw new Error("Failed to fetch tracks");
      tracks = (await res.json()) || [];
      navigate("library");
    } catch (e) {
      console.error("Genre filter failed", e);
    }
  }

  function handleSearchResultPlay(item) {
     handlePlay(item);
  }

  async function triggerScan() {
    try {
      const res = await fetch("/api/scan", { method: "POST" });
      if (res.ok) {
        showToast("Library scan started!", "success");
        // Reiniciar status local para activar el polling
        scanStatus = { scanning: true, processed: 0, inboxCount: 0 };
        startPolling();
        checkScanStatus();
      }
    } catch (e) {
      showToast("Scan failed", "error");
    }
  }

  async function openEditor(item) {
    if (item.coverId && !item.trackNum) {
      // It's an album (identified by coverId and absence of trackNum)
      try {
        const res = await fetch(`/api/albums/${item.id}/tracks`);
        if (!res.ok) throw new Error("Failed to fetch tracks");
        selectedTracks = await res.json();
        isMassiveEditorOpen = true;
      } catch (e) {
        showToast("Error loading album tracks: " + e.message, "error");
      }
    } else {
      // It's a single track
      trackToEdit = { ...item };
      isTagEditorOpen = true;
    }
  }

  function handlePlay(data) {
    let track, trackList;
    if (data.track) {
      track = data.track;
      trackList = data.tracks;
    } else {
      track = data;
    }

    if (trackList && trackList.length > 0) {
      queue.set(trackList);
      const idx = trackList.findIndex(t => t.id === track.id);
      currentIndex.set(idx >= 0 ? idx : 0);
    } else {
      // Single track play
      queue.set([track]);
      currentIndex.set(0);
    }
    isPlaying.set(true);
  }

  function handleShufflePlay(trackList) {
    if (!trackList || trackList.length === 0) return;
    const shuffled = [...trackList].sort(() => Math.random() - 0.5);
    queue.set(shuffled);
    currentIndex.set(0);
    isPlaying.set(true);
  }

  function handleSelectionChange(selected) {
    selectedTracks = selected;
  }

  async function handleDeletePhysical(track, callback) {
    openConfirm({
      title: "Eliminar archivo",
      message: `¡ADVERTENCIA! ¿Estás seguro de que quieres borrar FÍSICAMENTE "${track.title}" del disco? Esta acción no se puede deshacer.`,
      confirmLabel: "Borrar para siempre",
      type: "danger",
      onconfirm: async () => {
        try {
          const res = await fetch(`/api/tracks/${track.id}`, { method: 'DELETE' });
          if (res.ok) {
            showToast("Archivo eliminado del disco", "success");

            // Si la pista eliminada es la que está sonando, saltamos o detenemos
            if (currentPlaying && track.id === currentPlaying.id) {
              if (handleSkipNext) {
                handleSkipNext();
              } else {
                isPlaying.set(false);
              }
            }

            // También la eliminamos de la cola si está presente
            const q = get(queue);
            const foundIdx = q.findIndex(t => t.id === track.id);
            if (foundIdx !== -1) {
              const updatedQueue = q.filter(t => t.id !== track.id);
              queue.set(updatedQueue);
              
              const currentIdx = get(currentIndex);
              if (foundIdx < currentIdx) {
                currentIndex.set(currentIdx - 1);
              } else if (foundIdx === currentIdx && updatedQueue.length === 0) {
                isPlaying.set(false);
              }
            }

            fetchTracks();
            if (callback) callback();
          }
        } catch (e) {
          showToast("Error al eliminar el archivo", "error");
        }
      }
    });
  }

  async function handleReject(track, callback) {
    openConfirm({
      title: "Ocultar canción",
      message: `¿Ocultar "${track.title}" de la biblioteca? No aparecerá en Inbox ni Library, pero se conservará en tu disco.`,
      confirmLabel: "Ocultar",
      type: "info",
      icon: "eyeOff",
      onconfirm: async () => {
        try {
          const res = await fetch(`/api/tracks/reject`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ ids: [track.id] })
          });
          if (res.ok) {
            showToast("Canción ocultada", "success");
            fetchTracks();
            if (callback) callback();
          }
        } catch (e) {
           showToast("Error al ocultar", "error");
        }
      }
    });
  }

  async function handleBulkReject(ids, callback) {
    if (!ids || ids.length === 0) return;
    openConfirm({
      title: "Ocultar canciones",
      message: `¿Ocultar ${ids.length} canciones de la biblioteca? No aparecerán en Inbox ni Library, pero se conservarán en tu disco.`,
      confirmLabel: "Ocultar todas",
      type: "info",
      icon: "eyeOff",
      onconfirm: async () => {
        try {
          const res = await fetch('/api/tracks/reject', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ ids })
          });
          if (res.ok) {
            showToast(`${ids.length} canciones ocultadas`, "success");
            fetchTracks();
            if (callback) callback();
          }
        } catch (e) {
          showToast("Error al ocultar canciones", "error");
        }
      }
    });
  }

  function handleQueue(track) {
     showToast("Added to queue: " + track.title, "success");
     // Logic for adding to queue would go here
  }

  async function selectAlbum(albumId) {
    navigate("album", { albumId });
  }

  function handleSelectArtist(artist) {
    navigate("artist-detail", { artistId: artist.id });
  }

  async function handlePlayAlbum(albumId) {
    try {
      const res = await fetch(`/api/albums/${albumId}/tracks`);
      const albumTracks = (await res.json()) || [];
      if (albumTracks.length > 0) {
        handlePlay({ track: albumTracks[0], tracks: albumTracks });
      }
    } catch (e) { console.error(e); }
  }

  async function handleQueueAlbum(albumId) {
    try {
      const res = await fetch(`/api/albums/${albumId}/tracks`);
      const albumTracks = (await res.json()) || [];
      albumTracks.forEach(t => handleQueue(t));
      showToast(`${albumTracks.length} tracks added to queue`, "success");
    } catch (e) { console.error(e); }
  }

  function selectPlaylist(pl) {
    navigate("playlist", { playlist: pl });
  }

  function handleLogout() {
    localStorage.removeItem('apiKey');
    localStorage.removeItem('username');
    // Clear cookie (would requires server action or non-httponly, but we just trigger UI logout)
    document.cookie = "sgound_token=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;";
    isLoggedIn = false;
  }

  // --- Dynamic Title ---
  const viewTitles = {
    dashboard: "Dashboard",
    library: "Tracks",
    inbox: "Inbox",
    albums: "Albums",
    artists: "Artists",
    playlists: "Playlists",
    "smart-playlist": "Smart Playlist",
    playlist: "Playlist",
    album: "Album",
    "artist-detail": "Artist",
    settings: "Settings",
    maintenance: "Maintenance",
    fullplayer: "Player"
  };

  let pageTitle = $derived.by(() => {
    if (setupRequired) return "Setup | Sgound";
    if (!isLoggedIn) return "Login | Sgound";

    let base = "Sgound";
    let section = viewTitles[currentView] || "Web";
    
    if (currentView === 'playlist' && selectedPlaylist) {
      section = selectedPlaylist.name;
    } else if (currentView === 'smart-playlist' && selectedSmart) {
      section = selectedSmart.name;
    }

    if ($isPlaying && currentPlaying) {
      return `▶ ${currentPlaying.artist} - ${currentPlaying.title} | ${section}`;
    }
    
    return `${section} | ${base}`;
  });
</script>

<svelte:head>
  <title>{pageTitle}</title>
</svelte:head>

{#if setupRequired}
  <Setup 
    username={setupUsername} 
    on:complete={async () => { 
      setupRequired = false; 
      isLoggedIn = true; 
      // Esperar un poco para que el backend asiente el estado de escaneo
      await new Promise(r => setTimeout(r, 100));
      loadTheme();
      checkScanStatus();
      fetchTracks(); 
      fetchPlaylists(); 
    }} 
  />
{:else if !isLoggedIn}
  <Login 
    on:setup={(e) => { setupUsername = e.detail.username; setupRequired = true; }}
    on:login={() => {
      isLoggedIn = true;
      checkScanStatus();
      fetchTracks();
      fetchPlaylists();
      loadTheme();
    }} />{:else}
<main class="app-layout" class:is-cinema={currentView === 'fullplayer' && playerSubView === 'cinema'}>
  <aside class="sidebar">
    <div class="brand">
      <div class="logo">
        <div class="logo-icon"></div>
      </div>
      <h2>Sgound</h2>
    </div>



    <nav>
      <div class="nav-label">General</div>
      <button
        class="nav-item"
        class:active={currentView === "dashboard"}
        onclick={() => navigate("dashboard")}
      >
        <div class="nav-icon-wrap">
          <Icon name="grid" size={18} />
          {#if $inboxCount > 0 || $rawCount > 0}
            <span class="nav-notification-dot"></span>
          {/if}
        </div>
        <span>Dashboard</span>
        {#if $inboxCount > 0 || $rawCount > 0}
          <span class="nav-badge">{$inboxCount + $rawCount}</span>
        {/if}
      </button>

      {#if $inboxCount > 0 || $rawCount > 0}
        <button
          class="nav-item nav-inbox"
          class:active={currentView === "inbox"}
          onclick={() => navigate("inbox")}
          in:fade
        >
          <div class="nav-icon-wrap">
            <Icon name="inbox" size={18} />
          </div>
          <span>Inbox</span>
          <span class="nav-badge warning">{$inboxCount + $rawCount}</span>
        </button>
      {/if}

      <div class="nav-label">Library</div>
      <button
        class="nav-item"
        class:active={currentView === "library"}
        onclick={() => navigate("library")}
      >
        <Icon name="music" size={18} />
        <span>Tracks</span>
      </button>
      <button
        class="nav-item"
        class:active={currentView === "albums"}
        onclick={() => navigate("albums")}
      >
        <Icon name="disc" size={18} />
        <span>Albums</span>
      </button>
      <button
        class="nav-item"
        class:active={currentView === "artists"}
        onclick={() => navigate("artists")}
      >
        <Icon name="users" size={18} />
        <span>Artists</span>
      </button>
      <button
        class="nav-item"
        class:active={currentView === "playlists"}
        onclick={() => navigate("playlists")}
      >
        <Icon name="list" size={18} />
        <span>Playlists</span>
      </button>

      <div class="nav-label">Tools</div>
      <button
        class="nav-item"
        class:active={currentView === "maintenance"}
        onclick={() => navigate("maintenance")}
      >
        <Icon name="shield" size={18} />
        <span>Maintenance</span>
      </button>
      <button
        class="nav-item"
        class:active={currentView === "settings"}
        onclick={() => navigate("settings")}
      >
        <Icon name="settings" size={18} />
        <span>Settings</span>
      </button>

      <div class="nav-divider"></div>
      <div class="nav-label">Your Playlists</div>

      <div class="playlists-scroll">
        {#each playlists as pl}
          <button
            class="nav-item playlist-item"
            class:active={currentView === "playlist" &&
              selectedPlaylist?.id === pl.id}
            onclick={() => {
              selectedPlaylist = pl;
              navigate("playlist", { playlist: pl });
            }}
          >
            <div class="nav-icon-wrap">
              <Icon name={pl.isSmart ? "zap" : "list"} size={17} />
            </div>
            <span>{pl.name}</span>
            {#if pl.trackCount > 0}
              <span class="nav-badge">{pl.trackCount}</span>
            {/if}
          </button>
        {/each}
      </div>

      <div class="nav-divider"></div>
      <button
        class="nav-item logout-btn"
        onclick={handleLogout}
      >
        <Icon name="logOut" size={18} />
        <span>Cerrar Sesión</span>
      </button>
    </nav>
  </aside>

  <section class="content {currentView === 'fullplayer' ? 'full-player-view' : ''}">
    <header class="top-bar">
      <div class="search-box">
        <Search 
          onselect={handleSearchResultSelect} 
          onplay={handleSearchResultPlay}
          onsearch={handleSearch}
        />
      </div>
      <div class="actions">
        {#if selectedTracks.length > 1}
          <button
            class="bulk-btn"
            onclick={() => (isMassiveEditorOpen = true)}
          >
            Edit ({selectedTracks.length})
          </button>
        {/if}
      </div>
    </header>

    <div class="scroll-view" onscroll={handleTracksScroll}>
      {#if error}
        <div class="error-state">
          <Icon name="alertCircle" size={48} color="var(--primary)" />
          <p class="error-msg">{error}</p>
          <button class="retry-btn" onclick={() => fetchTracks()}>
            <Icon name="refreshCw" size={16} /> Reintentar carga
          </button>
        </div>
      {:else if currentView === "dashboard"}
        <Dashboard 
          bind:this={dashRef}
          onplay={handlePlay}
          onedit={openEditor}
          onqueue={handleQueue}
          onhide={(track) => handleReject(track, () => dashRef?.refresh())}
          ondeletePhysical={(track) => handleDeletePhysical(track, () => dashRef?.refresh())}
          onselectInbox={() => navigate("inbox")}
          onselectMaintenance={() => navigate("maintenance")}
          onselectPlaylist={selectPlaylist}
          onselectView={(view, id) => {
            if (view === 'tracks') navigate('library');
            else if (view === 'albums' && id) selectAlbum(id);
            else {
              navigate(view);
            }
          }}
          onsmartAlbums={() => { /* logic */ }}
          onsmartSearch={() => { /* logic */ }}
          onCleanupLibrary={() => { /* logic */ }}
          {scanStatus}
        />
      {:else if currentView === "library"}
        {#if tracks.length === 0}
          <div class="tracks-view">
            <header class="section-header">
              <div class="title-row">
                <h1>Tracks</h1>
              </div>
              <div class="chip primary" title="Total de pistas">
                <Icon name="music" size={14} />
                0
              </div>
            </header>
            <div class="empty-state">
              {#if scanStatus.scanning}
                <div class="spinner"></div>
                <p>Escaneando biblioteca... ({scanStatus.processed} procesados)</p>
              {/if}
              
              {#if $inboxCount > 0 || $rawCount > 0}
                <div class="inbox-alert">
                  <Icon name="inbox" size={48} />
                  <p>Tu biblioteca está vacía, pero tienes <b>{$inboxCount + $rawCount}</b> canciones en el Inbox pendientes de aprobar.</p>
                  <button class="primary-btn" onclick={() => navigate('inbox')}>Ir al Inbox</button>
                </div>
              {:else if !scanStatus.scanning}
                <div class="empty-icon"><Icon name="music" size={64} /></div>
                <p>No se han encontrado canciones. Haz clic en "Escanear Biblioteca" en Ajustes para comenzar.</p>
              {/if}
            </div>
          </div>
        {:else}
          <TracksView
            {tracks}
            {currentPlaying}
            loading={loadingTracks}
            onplay={(track) => handlePlay(track)}
            onedit={(track) => openEditor(track)}
            onselectionChange={(selection) => handleSelectionChange(selection)}
            onqueue={(track) => handleQueue(track)}
            onhide={handleReject}
            onbulkReject={handleBulkReject}
            ondeletePhysical={handleDeletePhysical}
            onshuffle={handleShufflePlay}
            onsearch={handleSearch}
          />
        {/if}
      {:else if currentView === "inbox"}
        <InboxView
          bind:this={inboxRef}
          onplay={(track) => handlePlay(track)}
          onedit={(track) => openEditor(track)}
          onapproved={() => { fetchTracks(false); checkScanStatus(); }}
          onrejected={checkScanStatus}
          onhide={(track) => handleReject(track, () => inboxRef?.fetchInbox())}
          onbulkReject={(ids) => handleBulkReject(ids, () => inboxRef?.fetchInbox())}
          ondeletePhysical={(track) => handleDeletePhysical(track, () => inboxRef?.fetchInbox())}
        />
      {:else if currentView === "albums"}
        <AlbumsGrid 
          artistId={selectedArtistId}
          onselectAlbum={(id) => navigate("tracks", { albumId: id })}
          onplayAlbum={handlePlayAlbum}
          onqueueAlbum={handleQueueAlbum}
          oneditAlbum={openEditor}
          onnavinbox={() => navigate('inbox')}
        />
      {:else if currentView === "artist-detail"}
        <ArtistDetail
          artistId={selectedArtistId}
          onselectAlbum={(id) => selectAlbum(id)}
          onplayAlbum={(id) => handlePlayAlbum(id)}
          onqueueAlbum={(id) => handleQueueAlbum(id)}
          oneditAlbum={(album) => openEditor(album)}
          onshuffle={handleShufflePlay}
        />
      {:else if currentView === "album"}
        <AlbumView
          albumId={selectedAlbumId}
          onplay={handlePlay}
          onedit={openEditor}
          onback={() => history.back()}
          onshuffle={handleShufflePlay}
        />
      {:else if currentView === "artists"}
        <ArtistsView
          onselectArtist={(artist) => navigate("artists", { artistId: artist.id, artistName: artist.name })}
          onnavinbox={() => navigate('inbox')}
        />
      {:else if currentView === "playlists"}
        <Playlists 
          on:selectPlaylist={(e) => selectPlaylist(e.detail)} 
          on:selectSmart={(e) => selectSmart(e.detail)}
        />
      {:else if currentView === "smart-playlist"}
        <SmartPlaylistView 
          bind:this={smartRef}
          {...selectedSmart}
          currentPlaying={currentPlaying}
          onback={() => history.back()}
          onplay={(track) => handlePlay(track)}
          onedit={(track) => openEditor(track)}
          onselectionChange={(selection) => handleSelectionChange(selection)}
          onhide={(track) => handleReject(track, () => smartRef?.fetchTracks())}
          ondeletePhysical={(track) => handleDeletePhysical(track, () => smartRef?.fetchTracks())}
          onshuffle={handleShufflePlay}
        />
      {:else if currentView === "playlist"}
        <PlaylistsView
          bind:this={playlistRef}
          playlist={selectedPlaylist}
          currentPlaying={currentPlaying}
          onplay={(track) => handlePlay(track)}
          onedit={(track) => openEditor(track)}
          onselectionChange={(selection) => handleSelectionChange(selection)}
          onhide={(track) => handleReject(track, () => playlistRef?.fetchPlaylistTracks())}
          ondeletePhysical={(track) => handleDeletePhysical(track, () => playlistRef?.fetchPlaylistTracks())}
          onqueue={(track) => handleQueue(track)}
          onshuffle={handleShufflePlay}
        />
      {:else if currentView === "settings"}
        <Settings onscan={triggerScan} />
      {:else if currentView === "maintenance"}
        <Maintenance 
          subview={selectedMaintenanceView} 
          onviewchange={(v) => navigate('maintenance', { subview: v })}
          onfixTags={() => { /* logic to open tag editor */ }} 
          onedit={(track) => openEditor(track)}
          bind:refresh={maintenanceRefresh}
        />
      {:else if currentView === "fullplayer"}
        <FullPlayer
          {currentPlaying}
          {audioRef}
          isOpen={true}
          bind:progress
          bind:duration
          bind:currentTime
          bind:subView={playerSubView}
          onedit={(track) => openEditor(track)}          onhide={handleReject}
          ondeletePhysical={handleDeletePhysical}
          onskipNext={() => handleSkipNext?.()}
          onskipBack={() => handleSkipBack?.()}
        />
      {/if}
    </div>
  </section>

  <div class="player-area">
    <div style="display: {currentView === 'fullplayer' ? 'contents' : 'none'}">
      <PlayerAnchorBar
        {currentPlaying}
        bind:view={playerSubView}
        oncloseFull={() => history.back()}
      />
    </div>
    <div style="display: {currentView !== 'fullplayer' ? 'contents' : 'none'}">
      <MiniPlayer
        {currentPlaying}
        bind:audioRef
        bind:progress
        bind:duration
        bind:currentTime
        bind:skipNext={handleSkipNext}
        bind:skipBack={handleSkipBack}
        onopenFull={() => navigate("fullplayer")}
        onedit={(track) => openEditor(track)}
        onhide={handleReject}
        ondeletePhysical={handleDeletePhysical}
        ontoggleQueue={() => isQueueOpen = !isQueueOpen}
        />
        </div>
        </div>
        </main>
        {/if}

        <QueuePanel bind:isOpen={isQueueOpen} />

        <TagEditor

  bind:isOpen={isTagEditorOpen}
  bind:track={trackToEdit}
  onclose={() => { fetchTracks(false); maintenanceRefresh?.(); }}
/>

<MassiveTagEditor
  bind:isOpen={isMassiveEditorOpen}
  {selectedTracks}
  onclose={() => { fetchTracks(false); maintenanceRefresh?.(); }}
/>

<Toast />

<ConfirmModal 
  bind:isOpen={confirmState.isOpen}
  title={confirmState.title}
  message={confirmState.message}
  confirmLabel={confirmState.confirmLabel}
  type={confirmState.type}
  icon={confirmState.icon}
  onconfirm={confirmState.onconfirm}
  oncancel={confirmState.oncancel}
/>
<style>
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
  }

  .sidebar {
    grid-area: sidebar;
    background: var(--surface);
    padding: 2rem 1.2rem;
    display: flex;
    flex-direction: column;
    align-items: flex-start; /* Fuerza alineación izquierda */
    z-index: 100;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 0.5rem 0.8rem;
    margin-bottom: 2rem;
  }

  .logo {
    width: 36px;
    height: 36px;
    background: var(--primary);
    border-radius: 12px;
    box-shadow: 0 4px 15px var(--primary-20);
    display: flex;
    align-items: center;
    justify-content: center;
    color: white;
  }

  .logo-icon {
    width: 22px;
    height: 22px;
    background-color: currentColor;
    -webkit-mask: url('/icons.svg?v=2') no-repeat center / contain;
    mask: url('/icons.svg?v=2') no-repeat center / contain;
  }

  .brand h2 { font-size: 1.5rem; margin: 0; font-weight: 900; letter-spacing: -0.8px; }

  nav { 
    display: flex; 
    flex-direction: column; 
    gap: 0.2rem; 
    flex: 1; 
    overflow-y: auto;
    scrollbar-width: none;
  }
  nav::-webkit-scrollbar { display: none; }

  .nav-label {
    padding: 1.5rem 0.8rem 0.5rem;
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.15em;
    color: var(--text-muted);
    font-weight: 800;
    opacity: 0.6;
  }

  .nav-item {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: flex-start; /* Contenido a la izq */
    gap: 1rem;
    width: 100%;
    padding: 0.75rem 1rem;
    background: transparent;
    border: none;
    color: var(--text-muted);
    text-align: left;
    border-radius: 12px;
    cursor: pointer;
    font-weight: 700;
    font-size: 0.92rem;
    transition: all 0.2s cubic-bezier(0.2, 0, 0, 1);
  }

  .nav-item:hover { background: var(--surface-container); color: var(--text-main); }
  .nav-item.active {
    background: var(--primary);
    color: #ffffff;
    box-shadow: 0 4px 15px var(--primary-20);
  }

  .nav-icon-wrap { position: relative; display: flex; align-items: center; justify-content: center; }
  .nav-badge {
    position: relative;
    background: var(--tertiary);
    color: black;
    font-size: 0.75rem;
    font-weight: 950;
    margin-left: auto;
    padding: 0.1rem 0.6rem;
    border-radius: 100px;
    box-shadow: 0 4px 12px rgba(0,0,0,0.2);
  }

  .nav-notification-dot {
    display: none;
    position: absolute;
    top: -2px;
    right: -2px;
    width: 10px;
    height: 10px;
    background: var(--tertiary);
    border-radius: 50%;
    border: 2px solid var(--surface);
    z-index: 20;
  }

  .playlists-scroll {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    padding-bottom: 2rem;
  }

  .playlist-item {
    padding-top: 0.6rem !important;
    padding-bottom: 0.6rem !important;
    font-size: 0.88rem !important;
  }

  .playlist-item span {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 160px;
  }

  .logout-btn { 
    margin-top: auto;
    color: #f87171 !important; 
  }
  .logout-btn:hover { background: rgba(248, 113, 113, 0.1) !important; }

  .content {
    grid-area: content;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: var(--surface-container);
    margin: 1rem 1rem 0.5rem 1rem;
    border-radius: 28px;
    border: 1px solid var(--border-color);
    box-shadow: 0 8px 32px rgba(0,0,0,0.12);
  }

  .content.full-player-view {
    margin-bottom: 0;
    margin-left: 1rem;
    margin-right: 1rem;
    border-bottom-left-radius: 0;
    border-bottom-right-radius: 0;
    border-bottom: none;
    background: var(--surface-container);
    box-shadow: none;
  }

  .player-area {
    grid-area: player;
    margin: 0.5rem 1rem 1rem 1rem; /* Aligned with content and sidebar */
    border-radius: 28px;
    display: flex;
    flex-direction: column;
    overflow: visible;
    background: var(--surface-container-highest);
    border: 1px solid var(--border-color);
    z-index: 10;
  }

  /* FullPlayer Unified Island on Desktop */
  .app-layout:has(.content.full-player-view) .player-area {
    margin: 0 1rem 1rem 1rem;
    border-top-left-radius: 0;
    border-top-right-radius: 0;
    border-top: none;
    background: var(--surface-container);
    width: auto;
    overflow: visible;
    z-index: 100;
    transition: all 0.4s ease;
  }

  /* Cinema Mode Overrides */
  .app-layout.is-cinema .player-area {
    background: transparent !important;
    border: none !important;
    box-shadow: none !important;
    margin: 0 !important;
  }

  .app-layout.is-cinema .sidebar,
  .app-layout.is-cinema .top-bar {
    display: none !important;
  }

  .app-layout.is-cinema .content {
    margin: 0 !important;
    padding: 0 !important;
    max-width: none !important;
    width: 100% !important;
    height: 100vh !important;
    border: none !important;
    border-radius: 0 !important;
  }

  .app-layout.is-cinema .scroll-view {
    padding: 0 !important;
    margin: 0 !important;
    height: 100vh !important;
    overflow: hidden !important;
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
    position: sticky;
    top: 0;
    z-index: 100;
  }
  .search-box { width: 400px; }

  @media (max-width: 768px) {
    .top-bar { padding: 1rem !important; }
  }


  .actions { display: flex; gap: 0.8rem; }

  .bulk-btn {
    background: var(--surface-container-high);
    color: var(--primary);
    border: 1px solid var(--primary-30);
    padding: 0.6rem 1.2rem;
    border-radius: 100px;
    font-weight: 700;
    cursor: pointer;
    transition: all 0.2s;
  }
  .bulk-btn:hover { background: var(--primary-10); }

  .scroll-view {
    flex: 1;
    overflow-y: auto;
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
    scrollbar-width: none;
    -ms-overflow-style: none;
  }
  .scroll-view::-webkit-scrollbar { display: none; }
  
  @media (max-width: 768px) {
    .scroll-view { padding: 1rem; }
  }
  .scroll-view::-webkit-scrollbar-track { background: transparent; }

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
    animation: fadeIn 0.4s ease;
  }
  .error-msg {
    color: #fca5a5;
    font-weight: 700;
    font-size: 1.1rem;
    margin: 0;
    max-width: 400px;
    line-height: 1.4;
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
    transition: all 0.2s cubic-bezier(0.2, 0, 0, 1);
    box-shadow: 0 4px 15px var(--primary-20);
  }
  .retry-btn:hover {
    transform: translateY(-2px);
    box-shadow: 0 8px 25px var(--primary-30);
    background: var(--primary-high);
  }
  .retry-btn:active {
    transform: translateY(0);
  }

  .scan-status-item {
    margin: 0.5rem 0 1.5rem 0;
    padding: 0.8rem 1rem;
    background: var(--primary-10);
    border-radius: 14px;
    display: flex;
    align-items: center;
    gap: 0.8rem;
    border: 1px solid var(--primary-20);
  }

  .scan-status-item .spinner {
    width: 22px;
    height: 22px;
    border: 3px solid var(--primary-20);
    border-top-color: var(--primary);
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }
  .scan-status-item .info {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .scan-status-item .label {
    font-size: 0.8rem;
    font-weight: 800;
    color: var(--primary);
    text-transform: uppercase;
    letter-spacing: 1px;
  }
  .scan-status-item .count {
    font-size: 0.75rem;
    color: var(--text-muted);
    font-weight: 600;
  }
  @keyframes spin { to { transform: rotate(360deg); } }
  @keyframes fadeIn { from { opacity: 0; transform: translateY(10px); } to { opacity: 1; transform: translateY(0); } }

  /* Responsive Adjustments */
  @media (max-width: 1400px) {
    .app-layout { grid-template-columns: 220px minmax(0, 1fr); }
    .sidebar { padding: 1.5rem 0.8rem; }
    .brand h2 { font-size: 1.2rem; }
  }

  @media (max-width: 1100px) {
    .app-layout { grid-template-columns: 80px minmax(0, 1fr); }
    .nav-label, .brand h2, .nav-item span, .nav-badge, .scan-status-item .info { display: none; }
    .brand { justify-content: center; padding: 1rem 0; width: 100%; }
    .sidebar { align-items: center; padding: 1.5rem 0; width: 80px; }
    .nav-item { 
      width: 48px; 
      height: 48px; 
      justify-content: center; 
      padding: 0; 
      margin: 0.4rem 0;
      border-radius: 16px; 
    }
    .nav-notification-dot { display: block !important; }
    .content { margin: 1rem 0.6rem 0.5rem 0.6rem; border-radius: 28px; }
    .player-area { margin: 0.5rem 0.6rem 1rem 0.6rem; border-radius: 28px; }
  }

  @media (max-width: 768px) {
    .app-layout {
      grid-template-areas: "content" "player" "sidebar";
      grid-template-columns: 1fr;
      grid-template-rows: 1fr auto auto;
      gap: 0;
    }
    .sidebar {
      position: static;
      width: auto !important;
      height: 65px !important;
      flex-direction: row !important;
      justify-content: center !important;
      align-items: center !important;
      border: 1px solid var(--border-color);
      border-top: none;
      padding: 0 !important;
      margin: 0 1rem 1rem 1rem !important; /* Joined with player at 1rem bottom */
      border-radius: 0 0 24px 24px !important;
      background: var(--surface-container-highest);
      box-shadow: 0 10px 30px rgba(0,0,0,0.3);
    }
    nav { 
      flex-direction: row !important; 
      width: 100%; 
      justify-content: space-around; 
      align-items: center;
      gap: 0;
    }
    .brand, .nav-label, .nav-divider, .playlists-scroll, .logout-btn { display: none !important; }
    .nav-item { margin: 0; width: 50px; height: 50px; }
    .content { margin: 1rem 1rem 0.5rem 1rem !important; border-radius: 24px; }
    .player-area { 
      margin: 0 1rem 0 1rem !important; 
      border-radius: 24px 24px 0 0 !important;
      background: var(--surface-container-highest); /* Match sidebar background */
      border: 1px solid var(--border-color);
      border-bottom: none;
    }
    .sidebar {
      margin-top: 0 !important;
      border-radius: 0 0 24px 24px !important;
      border-top: 1px solid rgba(255,255,255,0.03); /* Subtle internal division */
      background: var(--surface-container-high) !important;
    }
    .app-layout:has(.content.full-player-view) .content {
      margin-bottom: 0 !important;
      border-radius: 24px 24px 0 0 !important;
      border-bottom: none;
    }
    .app-layout:has(.content.full-player-view) .player-area {
      margin: 0 1rem 0 1rem !important;
      border-radius: 0 !important;
      border-top: 1px solid rgba(255,255,255,0.05);
      border-bottom: none;
    }
    .app-layout:has(.content.full-player-view) .sidebar {
      margin-top: 0 !important;
      border-radius: 0 0 24px 24px !important;
      border-top: none;
    }
  }
  .content.full-player-view {
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .content.full-player-view .scroll-view {
    padding-bottom: 0;
  }

  /* Target the anchor bar through global or context if needed, 
     but App.svelte controls the grid container too */
</style>
