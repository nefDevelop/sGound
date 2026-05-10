<script>
  import InboxView from "$lib/InboxView.svelte";
  import { getContext } from "svelte";
  
  const { handlers, currentPlaying } = getContext("app");
</script>

<InboxView
  {currentPlaying}
  onplay={handlers.handlePlay}
  onedit={handlers.openEditor}
  onqueue={handlers.handleQueue}
  onhide={handlers.handleReject}
  ondeletePhysical={handlers.handleDeletePhysical}
  onshuffle={handlers.handleShufflePlay}
/>
