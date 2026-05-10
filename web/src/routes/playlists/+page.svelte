<script>
  import Playlists from "$lib/Playlists.svelte";
  import { goto } from "$app/navigation";
</script>

<Playlists 
  on:selectPlaylist={(e) => goto(`/playlist/${e.detail.id}`)} 
  on:selectSmart={(e) => goto(`/smart-playlist/${e.detail.id}`)}
/>
