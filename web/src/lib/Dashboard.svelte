<script>
  import { onMount, createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';
  import ContextMenu from './ContextMenu.svelte';
  
  const dispatch = createEventDispatcher();
  let stats = $state({
    totalTracks: 0,
    totalAlbums: 0,
    totalArtists: 0,
    totalPlaylists: 0,
    inboxCount: 0,
    forgottenGems: [],
    mostPlayed: []
  });
  let health = $state({ missingTags: 0, missingLyrics: 0, duplicateCount: 0, orphanCount: 0 });
  let loading = $state(true);

  let contextMenu = $state({
    visible: false,
    x: 0,
    y: 0,
    track: null,
    items: []
  });

  function showContextMenu(track, e) {
    e.preventDefault();
    contextMenu = {
      visible: true,
      x: e.clientX,
      y: e.clientY,
      track,
      items: [
        { label: 'Reproducir', icon: 'play', action: () => dispatch('play', track) },
        { label: 'Añadir a cola', icon: 'plus', action: () => dispatch('queue', track) },
        { divider: true },
        { label: 'Editar etiquetas', icon: 'edit', action: () => dispatch('edit', track) },
        { divider: true },
        { label: 'Copiar título', icon: 'fileText', action: () => navigator.clipboard.writeText(track.title) },
        { label: 'Eliminar de biblioteca', icon: 'xCircle', color: 'danger', action: () => dispatch('delete', track) }
      ]
    };
  }

  export async function refresh() {
    try {
      const [sRes, hRes] = await Promise.all([
        fetch('/api/dashboard'),
        fetch('/api/library/health')
      ]);
      if (sRes.ok) {
        const data = await sRes.json();
        // Garantizar que siempre sean arrays
        data.forgottenGems = data.forgottenGems || [];
        data.mostPlayed = data.mostPlayed || [];
        stats = data;
      }
      if (hRes.ok) health = await hRes.json();
    } catch (e) { console.error(e); }
    finally { loading = false; }
  }

  onMount(() => {
    refresh();
  });
</script>

<div class="dashboard">
  <div class="bento-grid">
    <!-- Welcome Tile -->
    <section class="bento-item welcome-tile">
      <div class="welcome-content">
        <h1>Welcome back, <span>User</span></h1>
        <p>Tu biblioteca tiene {stats.totalTracks} canciones listas para sonar.</p>
      </div>
      <div class="quick-stats">
        <div class="stat"><Icon name="disc" size={14} /> {stats.totalAlbums} Albums</div>
        <div class="stat"><Icon name="user" size={14} /> {stats.totalArtists} Artists</div>
        <div class="stat"><Icon name="list" size={14} /> {stats.totalPlaylists} Playlists</div>
      </div>
    </section>

    <!-- Inbox Tile -->
    <button 
      class="bento-item inbox-tile" 
      class:has-pending={stats.inboxCount > 0} 
      onclick={() => dispatch('selectInbox')}
      aria-label="Ir al Inbox"
    >
      <div class="tile-header">
        <Icon name="zap" size={24} color={stats.inboxCount > 0 ? 'var(--tertiary)' : 'var(--text-muted)'} />
        <h3>Inbox</h3>
      </div>
      <div class="inbox-content">
        <span class="count">{stats.inboxCount}</span>
        <span class="label">{stats.inboxCount === 1 ? 'Pendiente' : 'Pendientes'}</span>
      </div>
      <div class="action-hint">Revisar <Icon name="chevronRight" size={14} /></div>
    </button>

    <!-- Health Tile -->
    <section class="bento-item health-tile">
      <div class="tile-header">
        <Icon name="shield" size={18} color="var(--secondary)" />
        <h3>Salud</h3>
      </div>
      <div class="health-summary">
        <div class="h-row" class:warning={health.missingTags > 0}>
          <span class="lbl">S/ Etiquetas</span>
          <span class="val">{health.missingTags}</span>
        </div>
        <div class="h-row" class:warning={health.orphanCount > 0}>
          <span class="lbl">Huérfanos</span>
          <span class="val">{health.orphanCount}</span>
        </div>
      </div>
      <button class="maint-btn" onclick={() => dispatch('selectMaintenance')}>Gestionar <Icon name="settings" size={12} /></button>
    </section>

    <!-- Top Plays Tile -->
    <section class="bento-item most-played-tile">
      <div class="tile-header">
        <Icon name="activity" size={20} color="var(--secondary)" />
        <h3>Top Plays</h3>
      </div>
      <div class="top-list scroll-shadow">
        {#each stats.mostPlayed as top}
          <button class="top-item" onclick={() => dispatch('play', top)} aria-label={`Reproducir ${top.title}`}>
            <img src={`/api/cover/${top.id}`} alt="" onerror={(e) => { e.currentTarget.style.opacity='0'; }} />
            <div class="top-info">
              <span class="title">{top.title}</span>
              <span class="artist">{top.artist}</span>
            </div>
            <span class="play-count">{(top.playCount || 0)} <small>repro</small></span>
          </button>
        {/each}
        {#if !stats.mostPlayed || stats.mostPlayed.length === 0}
          <div class="empty">Empieza a escuchar para ver tus tendencias</div>
        {/if}
      </div>
    </section>

    <!-- Gems Tile (3x3 Grid) -->
    <section class="bento-item gems-tile">
      <div class="tile-header">
          <Icon name="archive" size={20} color="var(--primary)" />
          <h3>Forgotten Gems</h3>
      </div>
      <div class="gems-grid">
        {#each stats.forgottenGems as gem}
          <button 
            class="gem-cover" 
            onclick={() => dispatch('play', gem)} 
            oncontextmenu={(e) => showContextMenu(gem, e)}
            title={gem.title}
          >
            <img src={`/api/cover/${gem.id}`} alt={gem.title} onerror={(e) => { e.currentTarget.style.opacity='0'; }} />
            <div class="overlay"><Icon name="play" size={14} color="white" /></div>
          </button>
        {/each}
      </div>
    </section>

    <!-- Discovery Tile -->
    <section class="bento-item discovery-tile">
      <div class="tile-header">
        <Icon name="compass" size={20} color="var(--tertiary)" />
        <h3>Descubrir</h3>
      </div>
      <div class="discovery-visual">
        <div class="asymmetric-covers">
          <div class="disco-cover dc1"></div>
          <div class="disco-cover dc2"></div>
          <div class="disco-cover dc3"></div>
        </div>
        <div class="discovery-chips">
          <button class="chip primary">Inspiración</button>
          <button class="chip">Chill</button>
          <button class="chip secondary">Recuerdos</button>
          <button class="chip tertiary">Focus</button>
          <button class="chip">Clásicos</button>
        </div>
      </div>
    </section>

    <!-- Trends Tile -->
    <section class="bento-item trends-tile">
      <div class="tile-header">
        <Icon name="flame" size={20} color="var(--tertiary)" />
        <h3>Trends</h3>
      </div>
      <div class="genre-bars">
        <div class="bar-item">
          <div class="bar-info"><span>Electrónica</span> <small>42%</small></div>
          <div class="progress"><div class="fill" style="width: 42%"></div></div>
        </div>
        <div class="bar-item">
          <div class="bar-info"><span>Jazz</span> <small>28%</small></div>
          <div class="progress"><div class="fill sec" style="width: 28%"></div></div>
        </div>
        <div class="bar-item">
          <div class="bar-info"><span>Lo-Fi</span> <small>15%</small></div>
          <div class="progress"><div class="fill ter" style="width: 15%"></div></div>
        </div>
      </div>
    </section>
  </div>

  <ContextMenu 
    bind:visible={contextMenu.visible}
    x={contextMenu.x}
    y={contextMenu.y}
    items={contextMenu.items}
    on:close={() => contextMenu.visible = false}
  />
</div>

<style>
  .dashboard { padding: 1rem 0; animation: fadeIn 0.5s ease; }
  @keyframes fadeIn { from { opacity: 0; transform: translateY(10px); } to { opacity: 1; transform: translateY(0); } }

  .bento-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    grid-auto-rows: minmax(180px, auto);
    gap: 1.5rem;
  }

  .bento-item {
    background: var(--surface-container-high);
    border-radius: 32px;
    padding: 1.8rem;
    border: 1px solid var(--border-color);
    transition: all 0.3s cubic-bezier(0.2, 0, 0, 1);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    text-align: left;
    color: inherit;
    font-family: inherit;
  }
  .bento-item:not(button) { cursor: default; }
  button.bento-item { cursor: pointer; }
  .bento-item:hover { border-color: var(--primary-30); transform: translateY(-4px); }

  .tile-header { display: flex; align-items: center; gap: 0.8rem; margin-bottom: 1.2rem; }
  .tile-header h3 { margin: 0; font-size: 1.1rem; font-weight: 800; letter-spacing: -0.2px; }

  /* Welcome Tile */
  .welcome-tile { grid-column: span 2; background: linear-gradient(135deg, var(--surface-container-high) 0%, var(--surface-container-highest) 100%); position: relative; }
  .welcome-tile::after { content: ''; position: absolute; top: -50%; right: -20%; width: 280px; height: 280px; background: var(--primary-10); filter: blur(60px); border-radius: 50%; z-index: 0; }
  .welcome-content { position: relative; z-index: 1; }
  .welcome-tile h1 { font-size: 2.1rem; font-weight: 900; letter-spacing: -1.2px; margin: 0 0 0.5rem; }
  .welcome-tile h1 span { color: var(--primary); }
  .welcome-tile p { color: var(--text-muted); font-weight: 600; font-size: 0.95rem; margin: 0; }
  .quick-stats { margin-top: auto; display: flex; gap: 0.8rem; position: relative; z-index: 1; }
  .stat { font-size: 0.75rem; font-weight: 800; color: var(--text-main); background: rgba(255,255,255,0.05); padding: 0.4rem 0.8rem; border-radius: 100px; display: flex; align-items: center; gap: 0.4rem; white-space: nowrap; }

  /* Inbox Tile */
  .inbox-tile.has-pending { background: linear-gradient(135deg, var(--surface-container-highest) 0%, var(--tertiary-10) 100%); border-color: var(--tertiary-30); }
  .inbox-content { display: flex; flex-direction: column; align-items: center; justify-content: center; flex: 1; }
  .inbox-content .count { font-size: 3rem; font-weight: 950; color: var(--tertiary); line-height: 1; }
  .inbox-content .label { font-size: 0.75rem; font-weight: 800; color: var(--text-muted); text-transform: uppercase; letter-spacing: 1px; margin-top: 0.4rem; }
  .action-hint { font-size: 0.7rem; font-weight: 800; color: var(--primary); display: flex; align-items: center; gap: 0.3rem; margin-top: auto; opacity: 0; transform: translateX(-10px); transition: all 0.3s; }
  .inbox-tile:hover .action-hint { opacity: 1; transform: translateX(0); }

  /* Top Plays Tile */
  .most-played-tile { grid-column: span 1; grid-row: span 2; }
  .top-list { display: flex; flex-direction: column; gap: 0.5rem; overflow-y: auto; flex: 1; padding-right: 0.4rem; }
  .top-item { background: transparent; border: none; display: flex; align-items: center; gap: 0.8rem; padding: 0.5rem; border-radius: 16px; cursor: pointer; transition: background 0.2s; text-align: left; width: 100%; color: inherit; }
  .top-item:hover { background: var(--surface-container-highest); }
  .top-item img { width: 40px; height: 40px; border-radius: 8px; object-fit: cover; }
  .top-info { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
  .top-info .title { font-weight: 800; font-size: 0.85rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; color: var(--text-main); }
  .top-info .artist { font-size: 0.7rem; color: var(--text-muted); font-weight: 700; }
  .play-count { font-size: 0.8rem; font-weight: 950; color: var(--secondary); text-align: right; line-height: 1.1; }
  .play-count small { font-size: 0.55rem; color: var(--text-muted); display: block; }
  .empty { font-size: 0.75rem; color: var(--text-muted); text-align: center; margin-top: 3rem; opacity: 0.5; font-weight: 600; }

  /* Gems Tile (3x3 Grid) */
  .gems-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 0.5rem; }
  .gem-cover { background: var(--surface-container-highest); padding: 0; border: 1px solid var(--border-color); aspect-ratio: 1/1; border-radius: 12px; overflow: hidden; cursor: pointer; position: relative; }
  .gem-cover img { width: 100%; height: 100%; object-fit: cover; }
  .gem-cover .overlay { position: absolute; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; opacity: 0; transition: opacity 0.2s; }
  .gem-cover:hover .overlay { opacity: 1; }

  /* Discovery Tile */
  .discovery-visual { flex: 1; display: flex; flex-direction: column; gap: 1rem; }
  .asymmetric-covers { position: relative; height: 90px; margin-top: 0.5rem; }
  .disco-cover { position: absolute; width: 64px; height: 64px; border-radius: 14px; background: var(--surface-container-highest); border: 2px solid var(--border-color); box-shadow: 0 8px 16px rgba(0,0,0,0.4); }
  .dc1 { top: 0; left: 5px; transform: rotate(-8deg); background: linear-gradient(45deg, var(--primary-20), transparent); }
  .dc2 { top: 10px; left: 40px; transform: rotate(5deg); z-index: 1; background: linear-gradient(45deg, var(--secondary-20), transparent); }
  .dc3 { top: 5px; left: 75px; transform: rotate(-5deg); background: linear-gradient(45deg, var(--tertiary-20), transparent); }
  .discovery-chips { display: flex; flex-wrap: wrap; gap: 0.4rem; margin-top: 0.5rem; }
  .chip { background: var(--surface-container-highest); border: 1px solid var(--border-color); color: var(--text-main); font-size: 0.68rem; font-weight: 800; padding: 0.4rem 0.8rem; border-radius: 100px; cursor: pointer; transition: all 0.2s; }
  .chip:hover { border-color: var(--primary); transform: scale(1.05); }
  .chip.primary { background: var(--primary-10); color: var(--primary); }
  .chip.secondary { background: var(--secondary-10); color: var(--secondary); }
  .chip.tertiary { background: var(--tertiary-10); color: var(--tertiary); }

  /* Trends Tile */
  .genre-bars { display: flex; flex-direction: column; gap: 0.8rem; }
  .bar-item { display: flex; flex-direction: column; gap: 0.3rem; }
  .bar-info { display: flex; justify-content: space-between; font-size: 0.75rem; font-weight: 800; }
  .bar-info small { color: var(--text-muted); }
  .progress { height: 5px; background: var(--surface-container-highest); border-radius: 10px; overflow: hidden; }
  .fill { height: 100%; background: var(--primary); border-radius: 10px; }
  .fill.sec { background: var(--secondary); }
  .fill.ter { background: var(--tertiary); }

  /* Health Tile */
  .health-summary { display: flex; flex-direction: column; gap: 0.5rem; margin-bottom: 0.8rem; }
  .h-row { display: flex; justify-content: space-between; font-size: 0.78rem; font-weight: 700; color: var(--text-muted); }
  .h-row .val { color: var(--text-main); font-weight: 950; }
  .h-row.warning .val { color: var(--secondary); }
  .maint-btn { margin-top: auto; background: var(--surface-container-highest); border: 1px solid var(--border-color); color: var(--text-muted); padding: 0.6rem; border-radius: 14px; font-weight: 800; font-size: 0.7rem; cursor: pointer; display: flex; align-items: center; justify-content: center; gap: 0.4rem; transition: all 0.2s; }
  .maint-btn:hover { border-color: var(--primary); color: var(--primary); }

  /* Scroll Utility */
  .scroll-shadow::-webkit-scrollbar { width: 4px; }
  .scroll-shadow::-webkit-scrollbar-thumb { background: var(--border-color); border-radius: 10px; }
</style>
