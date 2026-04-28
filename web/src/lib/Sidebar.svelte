<script>
  import Icon from './Icon.svelte';
  import { currentView, viewParams, inboxCount, rawCount, isLoggedIn } from './stores.js';

  let { playlists = [], onLogout } = $props();

  function navigate(view, params = {}) {
    currentView.set(view);
    viewParams.set(params);
    
    // Manage history
    const state = { view, params };
    if (!history.state || history.state.view !== view || JSON.stringify(history.state.params) !== JSON.stringify(params)) {
      history.pushState(state, "");
    }

    const scrollView = document.querySelector('.scroll-view');
    if (scrollView) scrollView.scrollTop = 0;
  }

  let hasPendingMusic = $derived($inboxCount > 0 || $rawCount > 0);
</script>

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
      class:active={$currentView === "dashboard"}
      onclick={() => navigate("dashboard")}
    >
      <div class="nav-icon-wrap">
        <Icon name="grid" size={18} />
        {#if hasPendingMusic}
          <span class="nav-notification-dot"></span>
        {/if}
      </div>
      <span>Dashboard</span>
    </button>

    <div class="nav-label">Library</div>
    <button
      class="nav-item"
      class:active={$currentView === "library" || $currentView === "tracks"}
      onclick={() => navigate("library")}
    >
      <Icon name="music" size={18} />
      <span>Tracks</span>
    </button>
    <button
      class="nav-item"
      class:active={$currentView === "albums" || $currentView === "album"}
      onclick={() => navigate("albums")}
    >
      <Icon name="disc" size={18} />
      <span>Albums</span>
    </button>
    <button
      class="nav-item"
      class:active={$currentView === "artists" || $currentView === "artist-detail"}
      onclick={() => navigate("artists")}
    >
      <Icon name="users" size={18} />
      <span>Artists</span>
    </button>
    <button
      class="nav-item"
      class:active={$currentView === "playlists" || $currentView === "playlist"}
      onclick={() => navigate("playlists")}
    >
      <Icon name="list" size={18} />
      <span>Playlists</span>
    </button>

    <div class="nav-label">Tools</div>
    <button
      class="nav-item"
      class:active={$currentView === "maintenance"}
      onclick={() => navigate("maintenance")}
    >
      <Icon name="shield" size={18} />
      <span>Maintenance</span>
    </button>
    <button
      class="nav-item"
      class:active={$currentView === "settings"}
      onclick={() => navigate("settings")}
    >
      <Icon name="settings" size={18} />
      <span>Settings</span>
    </button>

    <div class="nav-divider"></div>
    <div class="nav-label">Your Playlists</div>

    <div class="playlists-scroll">
      {#each playlists as pl}
        {@const currentPlId = $viewParams && $viewParams['playlist'] ? $viewParams['playlist'].id : null}
        <button
          class="nav-item playlist-item"
          class:active={$currentView === "playlist" && currentPlId === pl.id}
          onclick={() => navigate("playlist", { playlist: pl })}
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
      onclick={onLogout}
    >
      <Icon name="logOut" size={18} />
      <span>Logout</span>
    </button>
  </nav>
</aside>

<style>
  .sidebar {
    grid-area: sidebar;
    background: var(--surface);
    padding: 2rem 1.2rem;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    z-index: 100;
    height: 100%;
    overflow: hidden;
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
    width: 100%;
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
    justify-content: flex-start;
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
    display: block;
    position: absolute;
    top: -2px;
    right: -2px;
    width: 10px;
    height: 10px;
    background: #ff4d4f;
    border-radius: 50%;
    border: 2px solid var(--surface);
    z-index: 20;
    box-shadow: 0 0 10px rgba(255, 77, 79, 0.4);
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

  .nav-divider {
    height: 1px;
    background: var(--border-color);
    margin: 1rem 0;
    opacity: 0.5;
  }

  .logout-btn { 
    margin-top: auto;
    color: #f87171 !important; 
  }
  .logout-btn:hover { background: rgba(248, 113, 113, 0.1) !important; }

  @media (max-width: 1100px) {
    .nav-label, .brand h2, .nav-item span, .nav-badge { display: none; }
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
  }

  @media (max-width: 768px) {
    .sidebar {
      position: static;
      width: 100% !important;
      height: 65px !important;
      flex-direction: row !important;
      justify-content: center !important;
      align-items: center !important;
      border: 1px solid var(--border-color);
      border-radius: 0 0 24px 24px !important;
      background: var(--surface-container-high) !important;
      padding: 0 !important;
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
  }
</style>
