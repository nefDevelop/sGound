<script>
  import SmartPlaylistView from "$lib/SmartPlaylistView.svelte";
  import { getContext } from "svelte";
  import { page } from "$app/stores";
  
  const { handlers, currentPlaying } = getContext("app");
  const smartId = $derived($page.params.id);
</script>

<SmartPlaylistView 
  id={smartId}
  currentPlaying={currentPlaying}
  onback={() => history.back()}
  onplay={handlers.handlePlay}
  onedit={handlers.openEditor}
  onselectionChange={handlers.handleSelectionChange}
  onhide={handlers.handleReject}
  ondeletePhysical={handlers.handleDeletePhysical}
  onshuffle={handlers.handleShufflePlay}
/>
