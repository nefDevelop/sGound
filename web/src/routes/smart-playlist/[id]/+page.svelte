<script>
  import SmartPlaylistView from "$lib/SmartPlaylistView.svelte";
  import { getContext } from "svelte";
  import { page } from "$app/stores";
  
  const { handlers, currentPlaying } = getContext("app");
  const smartId = $derived($page.params.id);
</script>

<SmartPlaylistView 
  type={smartId}
  name={smartId.charAt(0).toUpperCase() + smartId.slice(1)}
  icon={smartId === 'recent' ? 'clock' : smartId === 'frequent' ? 'activity' : smartId === 'forgotten' ? 'archive' : 'list'}
  currentPlaying={currentPlaying}
  onback={() => history.back()}
  onplay={handlers.handlePlay}
  onedit={handlers.openEditor}
  onselectionChange={handlers.handleSelectionChange}
  onhide={handlers.handleReject}
  ondeletePhysical={handlers.handleDeletePhysical}
  onshuffle={handlers.handleShufflePlay}
/>
