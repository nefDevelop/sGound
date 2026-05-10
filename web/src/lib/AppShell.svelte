<script>
  import { fade } from "svelte/transition";
  import Sidebar from "./Sidebar.svelte";
  import MainContent from "./MainContent.svelte";
  import MiniPlayer from "./MiniPlayer.svelte";
  import PlayerAnchorBar from "./PlayerAnchorBar.svelte";
  import Search from "./Search.svelte";
  import Icon from "./Icon.svelte";
  import QueuePanel from "./QueuePanel.svelte";
  import { currentView, viewParams, inboxCount, rawCount } from "./stores.js";

  let { 
    isLoggedIn, 
    playlists,
    scanStatus,
    tracks,
    loadingTracks,
    currentPlaying,
    audioRef = $bindable(),
    progress = $bindable(),
    duration = $bindable(),
    currentTime = $bindable(),
    playerSubView = $bindable(),
    error,
    isMassiveEditorOpen = $bindable(),
    selectedTracks,
    handlers = {}
  } = $props();

  let isQueueOpen = $state(false);

  let hasPendingMusic = $derived($inboxCount > 0 || $rawCount > 0);
</script>

<main class="app-layout" class:is-cinema={$currentView === 'fullplayer' && playerSubView === 'cinema'}>
  <Sidebar {playlists} onLogout={handlers.handleLogout} />

  <section class="content" class:full-player-view={$currentView === 'fullplayer'}>
    {#if scanStatus.isPaused}
      <div class="ram-warning-banner">
        <Icon name="alertCircle" size={16} />
        <span><b>Memory usage critical ({scanStatus.ramUsedMB}MB).</b> Scanner paused.</span>
      </div>
    {/if}
    
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

    <MainContent 
      {tracks}
      {loadingTracks}
      {currentPlaying}
      {audioRef}
      {scanStatus}
      {progress}
      {duration}
      {currentTime}
      {playerSubView}
      {error}
      {handlers}
    />
  </section>

  <div class="player-area" class:is-fullplayer={$currentView === 'fullplayer'}>
    <div class="player-stack">
      {#if $currentView === 'fullplayer'}
        <div in:fade={{duration: 250, delay: 250}} out:fade={{duration: 250}}>
          <PlayerAnchorBar
            {currentPlaying}
            bind:view={playerSubView}
            oncloseFull={() => history.back()}
          />
        </div>
      {:else}
        <div in:fade={{duration: 250, delay: 250}} out:fade={{duration: 250}}>
          <MiniPlayer
            {currentPlaying}
            bind:audioRef
            bind:progress
            bind:duration
            bind:currentTime
            skipNext={handlers.handleSkipNext}
            skipBack={handlers.handleSkipBack}
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
      {/if}
    </div>
  </div>
</main>

<QueuePanel bind:isOpen={isQueueOpen} />

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
    transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .content.full-player-view {
    margin-bottom: 0;
    border-bottom-left-radius: 0;
    border-bottom-right-radius: 0;
    border-bottom: none;
    box-shadow: none;
  }

  .player-area {
    grid-area: player;
    margin: 0.5rem 1rem 1rem 1rem;
    border-radius: 28px;
    display: flex;
    flex-direction: column;
    background: var(--surface-container-highest);
    border: 1px solid var(--border-color);
    z-index: 10;
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

  .search-box { width: 400px; }
  
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

  @media (max-width: 1100px) {
    .app-layout { grid-template-columns: 80px minmax(0, 1fr); }
  }

  @media (max-width: 768px) {
    .app-layout {
      grid-template-areas: "content" "player" "sidebar";
      grid-template-columns: 1fr;
      grid-template-rows: 1fr auto auto;
    }
  }
</style>
