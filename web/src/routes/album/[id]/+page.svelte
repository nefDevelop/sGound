<script>
  import AlbumView from "$lib/AlbumView.svelte";
  import { getContext } from "svelte";
  import { page } from "$app/stores";
  
  const { handlers } = getContext("app");
  const albumId = $derived($page.params.id);
</script>

<AlbumView
  {albumId}
  onplay={handlers.handlePlay}
  onedit={handlers.openEditor}
  oneditAlbum={handlers.openAlbumEditor}
  onback={() => history.back()}
  onshuffle={handlers.handleShufflePlay}
/>
