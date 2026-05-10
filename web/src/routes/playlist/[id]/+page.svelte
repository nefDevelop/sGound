<script>
  import PlaylistsView from "$lib/PlaylistsView.svelte";
  import { getContext } from "svelte";
  import { page } from "$app/stores";
  
  const { handlers, currentPlaying } = getContext("app");
  const playlistId = $derived($page.params.id);
  // Note: We might need to fetch the playlist name here or update PlaylistsView to fetch it
</script>

<PlaylistsView
  playlist={{ id: playlistId, name: 'Loading...' }}
  currentPlaying={currentPlaying}
  onplay={handlers.handlePlay}
  onedit={handlers.openEditor}
  onselectionChange={handlers.handleSelectionChange}
  onhide={handlers.handleReject}
  ondeletePhysical={handlers.handleDeletePhysical}
  onqueue={handlers.handleQueue}
  onshuffle={handlers.handleShufflePlay}
/>
