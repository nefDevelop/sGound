<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import TracksView from './TracksView.svelte';
  import { fade } from 'svelte/transition';
  import { inboxCount, rawCount } from './stores.js';

  let { onplay, onedit, onapproved, onrejected, onhide, onbulkReject, ondeletePhysical, scanStatus } = $props();
  
  let tracks = $state([]);
  let loading = $state(true);
  let tracksOffset = $state(0);
  let hasMore = $state(true);
  const limit = 40; 
  let selectedTracks = $state([]);
  let suggestions = $state([]);
  let isAnalyzing = $state(false);
  
  // Conteo real sincronizado con el servidor para esta vista
  let totalServer = $state(0);
  let totalFromHeader = $state(0);
  
  // Lógica de conteo definitiva: Header > Store > Scanner > Local
  let totalTracks = $derived(Math.max(
    totalFromHeader,
    totalServer, 
    $inboxCount + $rawCount, 
    tracks.length,
    (scanStatus?.inboxCount || 0) + (scanStatus?.rawCount || 0)
  ));

  // LOGS DE DEPURACIÓN AMPLIADOS
  $effect(() => {
    console.group("🔍 DEBUG INBOX COUNTS");
    console.log("DATO MAESTRO (X-Total-Count Header):", totalFromHeader);
    console.log("Icono/Global Store ($inboxCount + $rawCount):", $inboxCount + $rawCount);
    console.log("Estado Escáner Prop (scanStatus):", (scanStatus?.inboxCount || 0) + (scanStatus?.rawCount || 0));
    console.log("Cargados físicamente (tracks.length):", tracks.length);
    console.log("Petición Status API (totalServer):", totalServer);
    console.log("--- RESULTADO FINAL EN BOTÓN ---");
    console.log("Valor calculado (totalTracks):", totalTracks);
    console.groupEnd();
  });

  onMount(() => {
    fetchInbox();
    
    // Escuchar scroll del contenedor global (.scroll-view)
    const scrollView = document.querySelector('.scroll-view');
    if (scrollView) {
      const handleScroll = () => {
        const { scrollTop, scrollHeight, clientHeight } = scrollView;
        if (scrollHeight - scrollTop <= clientHeight + 300 && !loading && hasMore) {
          fetchInbox(true, true);
        }
      };
      scrollView.addEventListener('scroll', handleScroll);
      return () => scrollView.removeEventListener('scroll', handleScroll);
    }
  });

  let pendingAnalysisCount = 0;

  export async function fetchInbox(append = false, silent = false, isFinal = false) {
    if (!silent) loading = true;
    if (!append) {
      tracksOffset = 0;
      hasMore = true;
      pendingAnalysisCount = 0;
    }

    try {
      // 1. Obtener las pistas del lote actual
      const res = await fetch(`/api/tracks?status=pending,raw&limit=${limit}&offset=${tracksOffset}`);
      if (res.ok) {
        // LEER HEADER MAESTRO DE CONTEO
        const countHeader = res.headers.get("X-Total-Count");
        if (countHeader) {
          totalFromHeader = parseInt(countHeader, 10);
        }

        const data = (await res.json()) || [];
        if (append) {
          tracks = [...tracks, ...data];
        } else {
          tracks = data;
        }

        if (data.length < limit) {
          hasMore = false;
        }
        tracksOffset += data.length;
        pendingAnalysisCount += data.length;

        // 2. REGLA: Analizar solo si tenemos un lote de 20+ o si es el aviso final
        if (pendingAnalysisCount >= 20 || isFinal) {
           analyzeInbox();
           pendingAnalysisCount = 0;
        }
      }

      // 3. Obtener el TOTAL real del servidor (Fallback/Sincronización)
      const statusRes = await fetch("/api/library/status");
      if (statusRes.ok) {
        const statusData = await statusRes.json();
        totalServer = (statusData.inboxCount || 0) + (statusData.rawCount || 0);
        inboxCount.set(statusData.inboxCount || 0);
        rawCount.set(statusData.rawCount || 0);
      }

    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  }

  async function approveAll() {
    let ids = tracks.map(t => t.id);
    
    // Si sabemos que hay más pistas de las que se ven, las pedimos todas al servidor antes de aprobar
    if (totalTracks > tracks.length || hasMore) {
      try {
        const fetchLimit = Math.max(totalTracks + 100, 2000);
        const res = await fetch(`/api/tracks?status=pending,raw&limit=${fetchLimit}&minimal=true`);
        if (res.ok) {
          const data = await res.json();
          ids = data.map(t => t.id);
        }
      } catch (e) {
        console.error("Error fetching all IDs for approval:", e);
      }
    }

    if (ids.length === 0) return;
    await approveIds(ids);
  }

  async function rejectAll() {
    let ids = tracks.map(t => t.id);
    
    if (totalTracks > tracks.length || hasMore) {
      try {
        const fetchLimit = Math.max(totalTracks + 100, 2000);
        const res = await fetch(`/api/tracks?status=pending,raw&limit=${fetchLimit}&minimal=true`);
        if (res.ok) {
          const data = await res.json();
          ids = data.map(t => t.id);
        }
      } catch (e) {
        console.error("Error fetching all IDs for rejection:", e);
      }
    }

    if (ids.length === 0) return;
    await rejectIds(ids);
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
    if (isAnalyzing) return;
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
        await fetchInbox(); 
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
        suggestions = suggestions.filter(s => s.trackId !== track.id);
        tracks = tracks.map(t => {
          if (t.id === track.id) {
            return { ...t, artist: sug.artist, year: sug.year, suggestedArtist: null, suggestedYear: null };
          }
          return t;
        });
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
    <div class="header-info">
      {#if totalTracks > 0}
        <div class="chip primary" title="Total de pistas en el Inbox">
          <Icon name="inbox" size={14} />
          {totalTracks} pistas
        </div>
      {/if}
      {#if $rawCount > 0}
        <div class="chip warning" title="Pistas descubiertas pendientes de analizar">
          <Icon name="loader" size={14} class="spin" />
          {$rawCount} analizando...
        </div>
      {/if}
      {#if hasMore}
        <div class="chip ghost" title="Mostrando parcial">
          Cargando de {limit} en {limit}...
        </div>
      {/if}
    </div>
    
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
          <button class="btn ghost" onclick={rejectAll} disabled={totalTracks === 0}>
            Reject All ({totalTracks})
          </button>
          <button class="btn ghost" onclick={approveAll} disabled={totalTracks === 0}>
            Approve All ({totalTracks})
          </button>
        {/if}
      </div>
  </header>

  <div class="inbox-content">
    {#if loading && tracks.length === 0}
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
      {#if loading && tracks.length > 0}
        <div class="loader-more">
           <Icon name="loader" size={24} class="spin" />
           <span>Cargando más pistas...</span>
        </div>
      {/if}
    {/if}
  </div>
</div>

<style>

  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 2.5rem;
    gap: 2rem;
  }

  h1 { font-size: 2.5rem; font-weight: 950; margin: 0; letter-spacing: -1.5px; color: var(--text-main); }

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

  .inbox-content { flex: 1; }

  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 5rem;
    color: var(--text-muted);
    gap: 1rem;
    text-align: center;
  }

  .loader-more {
    padding: 2rem;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1rem;
    color: var(--primary);
    font-weight: 700;
  }

  .spin {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }

  .header-info { display: flex; gap: 1rem; align-items: center; }
  .chip.ghost { background: var(--surface-container-highest); color: var(--text-muted); font-size: 0.75rem; border: 1px solid var(--border-color); }
</style>
