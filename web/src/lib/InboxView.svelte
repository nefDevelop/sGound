<script>
  import { onMount, createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';
  import TracksView from './TracksView.svelte';
  import { fade } from 'svelte/transition';
  import { inboxCount } from './stores.js';

  const dispatch = createEventDispatcher();
  
  let tracks = $state([]);
  let loading = $state(true);
  let selectedTracks = $state([]);

  onMount(fetchInbox);

  async function fetchInbox() {
    loading = true;
    try {
      const res = await fetch('/api/library/inbox');
      if (res.ok) {
        tracks = (await res.json()) || [];
        inboxCount.set(tracks.length);
      }
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  }

  async function approveAll() {
    if (tracks.length === 0) return;
    const ids = tracks.map(t => t.id);
    await approveIds(ids);
  }

  async function approveSelected() {
    if (selectedTracks.length === 0) return;
    const ids = selectedTracks.map(t => t.id);
    await approveIds(ids);
  }

  async function approveIds(ids) {
    try {
      const res = await fetch('/api/tracks/approve', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ids })
      });
      if (res.ok) {
        // Refresh
        await fetchInbox();
        selectedTracks = [];
        dispatch('approved');
      }
    } catch (e) {
      console.error(e);
    }
  }

  async function rejectIds(ids) {
    if (!confirm(`¿Estás seguro de que quieres rechazar ${ids.length} pistas? No aparecerán en tu biblioteca.`)) return;
    try {
      const res = await fetch('/api/tracks/reject', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ids })
      });
      if (res.ok) {
        // Refresh
        await fetchInbox();
        selectedTracks = [];
        dispatch('rejected');
      }
    } catch (e) {
      console.error(e);
    }
  }

  function handleSelection(e) {
    selectedTracks = e.detail;
  }
</script>

<div class="inbox-view" in:fade>
  <header class="view-header">
    <div class="texts">
      <h1>Bandeja de Entrada</h1>
      <p class="desc">Estos archivos han sido detectados recientemente. Valídalos para que aparezcan en tu biblioteca principal.</p>
    </div>
    
    <div class="actions">
       {#if selectedTracks.length > 0}
         <button class="btn danger" onclick={() => rejectIds(selectedTracks.map(t => t.id))}>
           <Icon name="trash" size={18} />
           Rechazar ({selectedTracks.length})
         </button>
         <button class="btn primary" onclick={approveSelected}>
           <Icon name="zap" size={18} />
           Aprobar Selección ({selectedTracks.length})
         </button>
       {:else}
         <button class="btn ghost" onclick={() => rejectIds(tracks.map(t => t.id))} disabled={tracks.length === 0}>
           Ignorar Todo
         </button>
         <button class="btn ghost" onclick={approveAll} disabled={tracks.length === 0}>
           Aprobar Todo ({tracks.length})
         </button>
       {/if}
    </div>
  </header>

  <div class="content">
    {#if loading}
      <div class="loading">Buscando nuevos archivos...</div>
    {:else if tracks.length === 0}
      <div class="empty">
        <Icon name="shield" size={64} />
        <p>Tu bandeja de entrada está vacía. ¡Toda tu música está en orden!</p>
      </div>
    {:else}
      <TracksView 
        {tracks} 
        on:selectionChange={handleSelection}
        on:play
        on:edit
      />
    {/if}
  </div>
</div>

<style>
  .inbox-view {
    height: 100%;
    display: flex;
    flex-direction: column;
  }

  .view-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-end;
    margin-bottom: 2rem;
    gap: 2rem;
  }

  h1 { font-family: var(--font-headline); font-size: 3rem; margin: 0; letter-spacing: -1.5px; }
  .desc { color: var(--text-muted); margin-top: 0.5rem; max-width: 600px; }

  .actions { display: flex; gap: 1rem; }

  .btn {
    display: flex;
    align-items: center;
    gap: 0.8rem;
    padding: 0.8rem 1.5rem;
    border-radius: 100px;
    font-weight: 700;
    cursor: pointer;
    transition: all 0.2s;
    border: none;
  }

  .btn.primary { background: var(--primary); color: white; box-shadow: 0 4px 12px var(--primary-10); }
  .btn.danger { background: #ff4444; color: white; }
  .btn.ghost { background: var(--surface-container-high); color: var(--text-main); }
  .btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .content { flex: 1; overflow-y: auto; }

  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 5rem;
    color: var(--text-muted);
    gap: 1rem;
    text-align: center;
  }
</style>
