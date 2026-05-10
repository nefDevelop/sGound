<script>
  import ArtistDetail from "$lib/ArtistDetail.svelte";
  import { getContext } from "svelte";
  import { page } from "$app/stores";
  import { goto } from "$app/navigation";
  
  const { handlers } = getContext("app");
  const artistId = $derived($page.params.id);
</script>

<ArtistDetail
  {artistId}
  onselectAlbum={(id) => goto(`/album/${id}`)}
  onplayAlbum={handlers.handlePlayAlbum}
  onqueueAlbum={handlers.handleQueueAlbum}
  oneditAlbum={handlers.openAlbumEditor}
  onshuffle={handlers.handleShufflePlay}
/>
