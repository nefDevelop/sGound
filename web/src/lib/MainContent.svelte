<script>
  import Dashboard from "./Dashboard.svelte";
  import TracksView from "./TracksView.svelte";
  import InboxView from "./InboxView.svelte";
  import AlbumsGrid from "./AlbumsGrid.svelte";
  import ArtistDetail from "./ArtistDetail.svelte";
  import AlbumView from "./AlbumView.svelte";
  import ArtistsView from "./ArtistsView.svelte";
  import Playlists from "./Playlists.svelte";
  import PlaylistsView from "./PlaylistsView.svelte";
  import Maintenance from "./Maintenance.svelte";
  import SmartPlaylistView from "./SmartPlaylistView.svelte";
  import Settings from "./Settings.svelte";
  import FullPlayer from "./FullPlayer.svelte";
  import Icon from "./Icon.svelte";
  import { currentView, viewParams, isPlaying } from "./stores.js";

  let { 
    tracks, 
    loadingTracks, 
    currentPlaying, 
    audioRef, 
    scanStatus,
    progress,
    duration,
    currentTime,
    playerSubView,
    error,
    handlers = {}
  } = $props();

  // Reference for child calls
  let dashRef = $state();
  let inboxRef = $state();
  let smartRef = $state();
  let playlistRef = $state();
  let maintenanceRefresh = $state();

  function navigate(view, params = {}) {
    currentView.set(view);
    viewParams.set(params);
  }

  function selectAlbum(albumId) {
    navigate("album", { albumId });
  }

  function selectPlaylist(pl) {
    navigate("playlist", { playlist: pl });
  }

  function selectSmart(config) {
    navigate("smart-playlist", { smart: config });
  }
</script>

<div class="scroll-view" onscroll={handlers.handleTracksScroll}>
  {#if error}
    <div class="error-state">
      <Icon name="alertCircle" size={48} color="var(--primary)" />
      <p class="error-msg">{error}</p>
      <button class="retry-btn" onclick={handlers.fetchTracks}>
        <Icon name="refreshCw" size={16} /> Reintentar carga
      </button>
    </div>
  {:else if $currentView === "dashboard"}
    <Dashboard 
      bind:this={dashRef}
      onplay={handlers.handlePlay}
      onedit={handlers.openEditor}
      onqueue={handlers.handleQueue}
      onhide={(track) => handlers.handleReject(track, () => dashRef?.refresh())}
      ondeletePhysical={(track) => handlers.handleDeletePhysical(track, () => dashRef?.refresh())}
      onselectInbox={() => navigate("inbox")}
      onselectMaintenance={() => navigate("maintenance")}
      onselectPlaylist={selectPlaylist}
      onsmartAlbums={() => navigate("albums")}
      onsmartSearch={(q) => handlers.handleSearch(q)}
      onCleanupLibrary={() => navigate("maintenance", { subview: "cleanup" })}
      onselectView={(view, id) => {
        if (view === 'tracks') navigate('library');
        else if (view === 'albums' && id) selectAlbum(id);
        else navigate(view);
      }}
      {scanStatus}
    />
  {:else if $currentView === "library"}
    <TracksView
      {tracks}
      {currentPlaying}
      loading={loadingTracks}
      onplay={handlers.handlePlay}
      onedit={handlers.openEditor}
      onselectionChange={handlers.handleSelectionChange}
      onqueue={handlers.handleQueue}
      onhide={handlers.handleReject}
      onbulkReject={handlers.handleBulkReject}
      ondeletePhysical={handlers.handleDeletePhysical}
      onshuffle={handlers.handleShufflePlay}
      onsearch={handlers.handleSearch}
    />
  {:else if $currentView === "inbox"}
    <InboxView
      bind:this={inboxRef}
      {scanStatus}
      onplay={handlers.handlePlay}
      onedit={handlers.openEditor}
      onapproved={() => { handlers.fetchTracks(false); handlers.checkScanStatus(); }}
      onrejected={handlers.checkScanStatus}
      onhide={(track) => handlers.handleReject(track, () => inboxRef?.fetchInbox())}
      onbulkReject={(ids, refresh) => handlers.handleBulkReject(ids, () => inboxRef?.fetchInbox(), refresh)}
      ondeletePhysical={(track) => handlers.handleDeletePhysical(track, () => inboxRef?.fetchInbox())}
    />
  {:else if $currentView === "albums"}
    <AlbumsGrid 
      artistId={$viewParams['artistId']}
      onselectAlbum={selectAlbum}
      onplayAlbum={handlers.handlePlayAlbum}
      onqueueAlbum={handlers.handleQueueAlbum}
      oneditAlbum={handlers.openEditor}
      onnavinbox={() => navigate('inbox')}
    />
  {:else if $currentView === "artist-detail"}
    <ArtistDetail
      artistId={$viewParams['artistId']}
      onselectAlbum={selectAlbum}
      onplayAlbum={handlers.handlePlayAlbum}
      onqueueAlbum={handlers.handleQueueAlbum}
      oneditAlbum={handlers.openEditor}
      onshuffle={handlers.handleShufflePlay}
    />
  {:else if $currentView === "album"}
    <AlbumView
      albumId={$viewParams['albumId']}
      onplay={handlers.handlePlay}
      onedit={handlers.openEditor}
      onback={() => history.back()}
      onshuffle={handlers.handleShufflePlay}
    />
  {:else if $currentView === "artists"}
    <ArtistsView
      onselectArtist={(artist) => navigate("artist-detail", { artistId: artist.id, artistName: artist.name })}
      onnavinbox={() => navigate('inbox')}
    />
  {:else if $currentView === "playlists"}
    <Playlists 
      on:selectPlaylist={(e) => selectPlaylist(e.detail)} 
      on:selectSmart={(e) => selectSmart(e.detail)}
    />
  {:else if $currentView === "smart-playlist"}
    <SmartPlaylistView 
      bind:this={smartRef}
      {...$viewParams['smart']}
      currentPlaying={currentPlaying}
      onback={() => history.back()}
      onplay={handlers.handlePlay}
      onedit={handlers.openEditor}
      onselectionChange={handlers.handleSelectionChange}
      onhide={(track) => handlers.handleReject(track, () => smartRef?.fetchTracks())}
      ondeletePhysical={(track) => handlers.handleDeletePhysical(track, () => smartRef?.fetchTracks())}
      onshuffle={handlers.handleShufflePlay}
    />
  {:else if $currentView === "playlist"}
    <PlaylistsView
      bind:this={playlistRef}
      playlist={$viewParams['playlist']}
      currentPlaying={currentPlaying}
      onplay={handlers.handlePlay}
      onedit={handlers.openEditor}
      onselectionChange={handlers.handleSelectionChange}
      onhide={(track) => handlers.handleReject(track, () => playlistRef?.fetchPlaylistTracks())}
      ondeletePhysical={(track) => handlers.handleDeletePhysical(track, () => playlistRef?.fetchPlaylistTracks())}
      onqueue={handlers.handleQueue}
      onshuffle={handlers.handleShufflePlay}
    />
  {:else if $currentView === "settings"}
    <Settings onscan={handlers.triggerScan} />
  {:else if $currentView === "maintenance"}
    <Maintenance 
      subview={$viewParams['subview'] || 'overview'} 
      onviewchange={(v) => navigate('maintenance', { subview: v })}
      onfixTags={handlers.handleFixTags} 
      onedit={handlers.openEditor}
      bind:refresh={maintenanceRefresh}
    />
  {:else if $currentView === "fullplayer"}
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
  {/if}
</div>

<style>
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
</style>
