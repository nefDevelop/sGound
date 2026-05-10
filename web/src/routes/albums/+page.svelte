<script>
  import AlbumsGrid from "$lib/AlbumsGrid.svelte";
  import { getContext } from "svelte";
  import { goto } from "$app/navigation";
  
  const { handlers } = getContext("app");
</script>

<AlbumsGrid 
  onselectAlbum={(id) => goto(`/album/${id}`)}
  onplayAlbum={handlers.handlePlayAlbum}
  onqueueAlbum={handlers.handleQueueAlbum}
  oneditAlbum={handlers.openAlbumEditor}
  onnavinbox={() => goto('/inbox')}
/>
