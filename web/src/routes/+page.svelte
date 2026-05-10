<script>
  import Dashboard from "$lib/Dashboard.svelte";
  import { goto } from "$app/navigation";
  import { currentView } from "$lib/stores.js";

  let { data } = $props();
  // We can get handlers and state from layout via a store or context
  // For now, let's assume we use stores or the components handle their own state
</script>

<Dashboard 
  onselectInbox={() => goto("/inbox")}
  onselectMaintenance={() => goto("/maintenance")}
  onselectPlaylist={(pl) => goto(`/playlist/${pl.id}`)}
  onsmartAlbums={() => goto("/albums")}
  onselectView={(view, id) => {
    if (view === 'tracks') goto('/library');
    else if (view === 'albums' && id) goto(`/album/${id}`);
    else goto(`/${view}`);
  }}
/>
