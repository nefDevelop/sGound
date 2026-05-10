<script>
  import Settings from "$lib/Settings.svelte";
  import { getContext } from "svelte";
  
  const { handlers } = getContext("app");
</script>

<Settings onscan={handlers.triggerScan} />
