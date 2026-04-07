<script>
  import { onMount, createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';
  
  const dispatch = createEventDispatcher();
  let stats = {
    totalTracks: 0,
    totalAlbums: 0,
    totalArtists: 0,
    totalPlaylists: 0,
    forgottenGems: []
  };
  let health = { missingTags: 0, missingLyrics: 0 };
  let loading = true;

  onMount(async () => {
    try {
      const [sRes, hRes] = await Promise.all([
        fetch('/api/dashboard'),
        fetch('/api/library/health')
      ]);
      stats = await sRes.json();
      health = await hRes.json();
    } catch (e) { console.error(e); }
    finally { loading = false; }
  });
</script>

<div class="dashboard">
  <header class="dash-header">
    <h1>Welcome back to <span>Sgound</span></h1>
    <div class="chips-row">
      <div class="chip"><Icon name="music" size={14} color="#ff3366" /> {stats.totalTracks} Songs</div>
      <div class="chip"><Icon name="disc" size={14} color="#ff3366" /> {stats.totalAlbums} Albums</div>
      <div class="chip"><Icon name="user" size={14} color="#ff3366" /> {stats.totalArtists} Artists</div>
      <div class="chip"><Icon name="list" size={14} color="#ff3366" /> {stats.totalPlaylists} Playlists</div>
    </div>
  </header>

  <div class="dash-grid">
    <!-- Joyas Olvidadas -->
    <section class="dash-card gems-card">
      <div class="card-header">
        <h3><Icon name="sparkles" size={18} color="#ffcc00" /> Forgotten Gems</h3>
        <button class="text-btn">Shuffle All</button>
      </div>
      <div class="gems-list">
        {#each stats.forgottenGems as gem}
          <!-- svelte-ignore a11y-click-events-have-key-events -->
          <!-- svelte-ignore a11y-no-static-element-interactions -->
          <div class="gem-item" on:click={() => dispatch('play', gem)}>
            <img src={`/api/cover/${gem.id}`} alt="" on:error={(e) => (e.currentTarget.style.display='none')} />
            <div class="gem-info">
              <span class="title">{gem.title}</span>
              <span class="artist">{gem.artist}</span>
            </div>
          </div>
        {/each}
      </div>
    </section>

    <!-- Saneamiento de Biblioteca -->
    <section class="dash-card health-card">
      <div class="card-header">
        <h3><Icon name="shield" size={18} color="#10b981" /> Library Health</h3>
      </div>
      <div class="health-stats">
        <div class="health-item" class:warning={health.missingTags > 0}>
          <span class="val">{health.missingTags}</span>
          <span class="lbl">Missing Genre/Year</span>
          <button class="fix-btn">Fix Now</button>
        </div>
        <div class="health-item">
          <span class="val">{health.missingLyrics}</span>
          <span class="lbl">Missing Lyrics</span>
          <button class="fix-btn">Auto-Download</button>
        </div>
      </div>
    </section>

    <!-- Categorías Más Escuchadas -->
    <section class="dash-card charts-card">
      <div class="card-header">
        <h3><Icon name="flame" size={18} color="#ff4400" /> Top Genres</h3>
      </div>
      <div class="genres-pills">
        <div class="pill-bar" style="width: 80%">Rock <span class="perc">80%</span></div>
        <div class="pill-bar" style="width: 60%; background: #6366f1">Electronic <span class="perc">60%</span></div>
        <div class="pill-bar" style="width: 40%; background: #10b981">Jazz <span class="perc">40%</span></div>
      </div>
    </section>
  </div>
</div>

<style>
  .dashboard { padding: 1rem 0; animation: fadeIn 0.5s ease; }
  @keyframes fadeIn { from { opacity: 0; transform: translateY(10px); } to { opacity: 1; transform: translateY(0); } }

  .dash-header { margin-bottom: 3rem; }
  .dash-header h1 { font-size: 2.5rem; font-weight: 800; margin-bottom: 1.5rem; }
  .dash-header h1 span { color: #ff3366; }

  .chips-row { display: flex; gap: 1rem; flex-wrap: wrap; }
  .chip {
    background: #1e293b; color: white; padding: 0.6rem 1.2rem;
    border-radius: 100px; font-weight: 600; font-size: 0.9rem;
    border: 1px solid rgba(255,255,255,0.05); display: flex; align-items: center; gap: 0.6rem;
  }

  .dash-grid {
    display: grid; grid-template-columns: 1fr 350px; grid-template-rows: auto auto; gap: 2rem;
  }

  .dash-card {
    background: #161b2c; border-radius: 32px; padding: 2rem;
    border: 1px solid rgba(255,255,255,0.03);
  }

  .card-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem; }
  .card-header h3 { margin: 0; font-size: 1.2rem; font-weight: 700; }

  .gems-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(250px, 1fr)); gap: 1rem; }
  .gem-item {
    background: rgba(255,255,255,0.03); padding: 1rem; border-radius: 20px;
    display: flex; align-items: center; gap: 1rem; cursor: pointer; transition: 0.2s;
  }
  .gem-item:hover { background: rgba(255,51,102,0.1); transform: scale(1.02); }
  .gem-item img { width: 48px; height: 48px; border-radius: 12px; object-fit: cover; }
  .gem-info { display: flex; flex-direction: column; overflow: hidden; }
  .gem-info .title { font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .gem-info .artist { font-size: 0.8rem; color: #64748b; }

  .health-stats { display: flex; flex-direction: column; gap: 1.5rem; }
  .health-item {
    background: rgba(0,0,0,0.2); padding: 1.5rem; border-radius: 24px;
    display: flex; flex-direction: column; align-items: center; text-align: center; gap: 0.5rem;
  }
  .health-item.warning { border: 1px solid rgba(255, 204, 0, 0.2); }
  .health-item .val { font-size: 2rem; font-weight: 800; color: #ff3366; }
  .health-item .lbl { font-size: 0.85rem; color: #94a3b8; margin-bottom: 0.5rem; }
  
  .fix-btn {
    background: #ff3366; color: white; border: none; padding: 0.5rem 1rem;
    border-radius: 100px; font-weight: 700; font-size: 0.75rem; cursor: pointer;
  }

  .genres-pills { display: flex; flex-direction: column; gap: 1rem; }
  .pill-bar {
    background: #ff3366; color: white; padding: 0.8rem 1.5rem; border-radius: 100px;
    font-weight: 700; font-size: 0.9rem; display: flex; justify-content: space-between;
  }
  .perc { opacity: 0.7; }

  .text-btn { background: transparent; border: none; color: #ff3366; font-weight: 700; cursor: pointer; }
</style>
