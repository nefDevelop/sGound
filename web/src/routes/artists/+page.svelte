<script>
  import ArtistsView from "$lib/ArtistsView.svelte";
  import { goto } from "$app/navigation";
</script>

<ArtistsView
  onselectArtist={(artist) => goto(`/artist/${artist.id}`)}
  onnavinbox={() => goto('/inbox')}
/>
