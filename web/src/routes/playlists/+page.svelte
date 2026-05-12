<script>
  import Playlists from "$lib/Playlists.svelte";
  import { goto } from "$app/navigation";
</script>

<Playlists 
  on:selectPlaylist={(/** @type {any} */ e) => goto(`/playlist/${e.detail.id}`)} 
  on:selectSmart={(/** @type {any} */ e) => goto(`/smart-playlist/${e.detail.type}`)}
/>
