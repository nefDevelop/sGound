<script>
  import Dashboard from "$lib/Dashboard.svelte";
  import { goto } from "$app/navigation";
  import { currentView } from "$lib/stores.js";
  import { getContext } from "svelte";

  let { data } = $props();
  const { handlers, currentPlaying } = getContext("app");
</script>

<Dashboard 
  onplay={handlers.handlePlay}
  onqueue={handlers.handleQueue}
  onedit={handlers.openEditor}
  onhide={handlers.handleReject}
  ondeletePhysical={handlers.handleDeletePhysical}
  onselectInbox={() => goto("/inbox")}
  onselectMaintenance={() => goto("/maintenance")}
  onselectPlaylist={(/** @type {any} */ pl) => goto(`/playlist/${pl.id}`)}
  onsmartAlbums={() => goto("/albums")}
  onselectView={(/** @type {any} */ view, /** @type {any} */ id) => {
    if (view === 'tracks') goto('/library');
    else if (view === 'albums' && id) goto(`/album/${id}`);
    else goto(`/${view}`);
  }}
/>
