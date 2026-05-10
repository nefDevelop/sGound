<script>
  import TracksView from "$lib/TracksView.svelte";
  import { getContext } from "svelte";
  
  const { handlers, tracks, loadingTracks, currentPlaying } = getContext("app");
</script>

<TracksView
  tracks={tracks}
  currentPlaying={currentPlaying}
  loading={loadingTracks}
  onplay={handlers.handlePlay}
  onedit={handlers.openEditor}
  onselectionChange={handlers.handleSelectionChange}
  onqueue={handlers.handleQueue}
  onhide={handlers.handleReject}
  onbulkReject={handlers.handleBulkReject}
  ondeletePhysical={handlers.handleDeletePhysical}
  onshuffle={handlers.handleShufflePlay}
/>
