<script>
  import Maintenance from "$lib/Maintenance.svelte";
  import { getContext } from "svelte";
  import { page } from "$app/stores";
  import { goto } from "$app/navigation";
  
  const { handlers } = getContext("app");
  const subview = $derived($page.url.searchParams.get('subview') || 'overview');
</script>

<Maintenance 
  {subview} 
  onviewchange={(/** @type {any} */ v) => goto(`/maintenance?subview=${v}`)}
  onfixTags={handlers.handleFixTags} 
  onedit={handlers.openEditor}
/>
