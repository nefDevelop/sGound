<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import TracksView from './TracksView.svelte';
  import { fade } from 'svelte/transition';
  import { inboxCount } from './stores.js';

  let { onplay, onedit, onapproved, onrejected, onhide, onbulkReject, ondeletePhysical } = $props();
  
  let tracks = $state([]);
  let loading = $state(true);
  let selectedTracks = $state([]);
  let suggestions = $state([]);
  let isAnalyzing = $state(false);

  onMount(fetchInbox);

  export async function fetchInbox() {
    loading = true;
    try {
      const res = await fetch('/api/library/inbox');
      if (res.ok) {
        tracks = (await res.json()) || [];
        inboxCount.set(tracks.length);
        if (tracks.length > 0) analyzeInbox();
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
        await fetchInbox();
        selectedTracks = [];
        onapproved?.();
      }
    } catch (e) {
      console.error(e);
    }
  }

  async function rejectIds(ids) {
    onbulkReject?.(ids);
  }

  async function analyzeInbox() {
    isAnalyzing = true;
    try {
      const res = await fetch('/api/library/inbox/analyze');
      if (res.ok) {
        const data = await res.json();
        suggestions = data.suggestions || [];
        // Aplicamos las sugerencias al estado local para visualización
        tracks = tracks.map(t => {
          const s = suggestions.find(sug => sug.trackId === t.id);
          if (s) {
            return { ...t, suggestedArtist: s.artist, suggestedYear: s.year };
          }
          return t;
        });
      }
    } catch (e) {
      console.error(e);
    } finally {
      isAnalyzing = false;
    }
  }

  async function applySuggestions() {
    if (suggestions.length === 0) return;
    try {
      const res = await fetch('/api/library/inbox/apply-suggestions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ suggestions })
      });
      if (res.ok) {
        suggestions = [];
        await fetchInbox(); // Recargamos para ver los cambios "oficiales"
      }
    } catch (e) {
      console.error(e);
    }
  }

  async function applySingleSuggestion(track) {
    const sug = suggestions.find(s => s.trackId === track.id);
    if (!sug) return;
    
    try {
      const res = await fetch('/api/library/inbox/apply-suggestions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ suggestions: [sug] })
      });
      if (res.ok) {
        // Quitamos la sugerencia de la lista local
        suggestions = suggestions.filter(s => s.trackId !== track.id);
        await fetchInbox();
      }
    } catch (e) {
      console.error(e);
    }
  }
</script>

<div class="inbox-view" in:fade>
  <header class="section-header">
    <div class="title-row">
      <h1>Inbox</h1>
    </div>
    {#if tracks.length > 0}
      <div class="chip primary" title="Total de pistas en el Inbox">
        <Icon name="inbox" size={14} />
        {tracks.length}
      </div>
    {/if}
    
      <div class="actions">
        {#if suggestions.length > 0}
          <button class="btn magic" onclick={applySuggestions}>
            <Icon name="zap" size={18} />
            Aplicar Cambios ({suggestions.length})
          </button>
        {/if}

        {#if selectedTracks.length > 0}
          <button class="btn danger" onclick={() => rejectIds(selectedTracks.map(t => t.id))}>
            <Icon name="trash" size={18} />
            Reject ({selectedTracks.length})
          </button>
          <button class="btn primary" onclick={approveSelected}>
            <Icon name="zap" size={18} />
            Approve Selected ({selectedTracks.length})
          </button>
        {:else}
          <button class="btn ghost" onclick={() => rejectIds(tracks.map(t => t.id))} disabled={tracks.length === 0}>
            Reject All
          </button>
          <button class="btn ghost" onclick={approveAll} disabled={tracks.length === 0}>
            Approve All ({tracks.length})
          </button>
        {/if}
      </div>
  </header>

  <div class="content">
    {#if loading}
      <div class="status">Sourcing new files...</div>
    {:else if tracks.length === 0}
      <div class="empty-state">
        <div class="empty-icon"><Icon name="shield" size={64} /></div>
        <p>No se han encontrado canciones en el Inbox. Haz clic en "Escanear Biblioteca" en Ajustes para comenzar.</p>
      </div>
    {:else}
      <TracksView 
        {tracks} 
        onselectionChange={(sel) => selectedTracks = sel}
        onplay={onplay}
        onedit={onedit}
        onqueue={(track) => {}}
        onhide={(track) => rejectIds([track.id])}
        onbulkReject={onbulkReject}
        ondeletePhysical={(track) => ondeletePhysical(track, fetchInbox)}
        onapplySuggestions={applySingleSuggestion}
        isInbox={true}
        showHeader={false}
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

  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 2.5rem;
    gap: 2rem;
  }

  h1 { font-size: 2.5rem; font-weight: 950; margin: 0; letter-spacing: -1.5px; color: var(--text-main); }
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
  .btn.magic { background: var(--primary); color: white; box-shadow: 0 4px 12px var(--primary-10); animation: pulse 2s infinite; }
  .btn.danger { background: #ff4444; color: white; }
  .btn.ghost { background: var(--surface-container-high); color: var(--text-main); }
  .btn:disabled { opacity: 0.5; cursor: not-allowed; }

  @keyframes pulse {
    0% { transform: scale(1); }
    50% { transform: scale(1.05); }
    100% { transform: scale(1); }
  }

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
