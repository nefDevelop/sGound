<script>
  import { onMount } from 'svelte';
  import { fade } from 'svelte/transition';
  import Icon from './Icon.svelte';
  import { showToast } from './stores.js';
  import StarRating from './StarRating.svelte';

  let { isOpen = $bindable(false), onCreated = null } = $props();

  let name = $state("");
  let isSmart = $state(false);
  let loading = $state(false);
  let genres = $state([]);
  
  let rules = $state({
    genre: "",
    minRating: 0,
    lastPlayedDays: 0,
    instrumental: null
  });

  onMount(async () => {
    try {
      const res = await fetch('/api/library/metadata');
      const data = await res.json();
      genres = data.genres || [];
    } catch (e) {
      console.error(e);
    }
  });

  async function handleCreate() {
    if (!name) {
      showToast("Please enter a name", "error");
      return;
    }
    
    loading = true;
    try {
      const payload = {
        name,
        isSmart,
        rules: isSmart ? rules : undefined
      };
      
      const res = await fetch('/api/playlists', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
      
      if (res.ok) {
        showToast("Playlist created!", "success");
        isOpen = false;
        reset();
        if (onCreated) onCreated();
      } else {
        const err = await res.text();
        showToast(err, "error");
      }
    } catch (e) {
      showToast("Network error", "error");
    } finally {
      loading = false;
    }
  }

  function reset() {
    name = "";
    isSmart = false;
    rules = { genre: "", minRating: 0, lastPlayedDays: 0, instrumental: null };
  }

  function closeModal() {
    isOpen = false;
    reset();
  }
</script>

{#if isOpen}
<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="modal-backdrop" onclick={closeModal}>
  <div class="modal glass" onclick={(e) => e.stopPropagation()}>
    <div class="modal-header">
      <h2>New Playlist</h2>
      <button class="close-btn" onclick={closeModal}><Icon name="x" size={20} /></button>
    </div>

    <div class="modal-content">
      <div class="form-group">
        <label for="pl-name">Playlist Name</label>
        <input id="pl-name" type="text" bind:value={name} placeholder="Epic Jazz Vibes..." />
      </div>

      <div class="smart-toggle">
        <label class="switch-row">
          <div class="texts">
            <span class="main-label" id="smart-pl-label">Smart Playlist</span>
            <span class="sub-label">Automatically select tracks based on rules.</span>
          </div>
          <input type="checkbox" bind:checked={isSmart} aria-labelledby="smart-pl-label" />
        </label>
      </div>

      {#if isSmart}
        <div class="rules-section" in:fade>
          <h3>Dynamic Rules</h3>
          
          <div class="rules-grid">
            <div class="rule">
              <label for="rule-genre">Genre</label>
              <select id="rule-genre" bind:value={rules.genre}>
                <option value="">Any Genre</option>
                {#each genres as g}
                  <option value={g}>{g}</option>
                {/each}
              </select>
            </div>

            <div class="rule">
              <label for="rule-rating">Minimum Rating</label>
              <div class="rating-wrap" id="rule-rating">
                <StarRating rating={rules.minRating} onchange={(r) => rules.minRating = r} size={24} />
                <span class="rating-label">{rules.minRating > 0 ? rules.minRating + '+' : 'Any'} stars</span>
              </div>
            </div>

            <div class="rule">
              <label for="rule-last-played">Last Played</label>
              <select id="rule-last-played" bind:value={rules.lastPlayedDays}>
                <option value={0}>Any time</option>
                <option value={30}>Not in the last month</option>
                <option value={90}>Not in the last 3 months</option>
                <option value={180}>Not in the last 6 months</option>
                <option value={365}>Not in the last year</option>
              </select>
            </div>

            <div class="rule">
              <span class="rule-label">Vocals</span>
              <div class="radio-group">
                <button type="button" class:active={rules.instrumental === null} onclick={() => rules.instrumental = null}>All</button>
                <button type="button" class:active={rules.instrumental === true} onclick={() => rules.instrumental = true}>Instru.</button>
                <button type="button" class:active={rules.instrumental === false} onclick={() => rules.instrumental = false}>Vocals</button>
              </div>
            </div>
          </div>
        </div>
      {/if}
    </div>

    <div class="modal-footer">
      <button class="cancel-btn" onclick={closeModal}>Cancel</button>
      <button class="save-btn" onclick={handleCreate} disabled={loading}>
        {loading ? 'Creating...' : 'Create Playlist'}
      </button>
    </div>
  </div>
</div>
{/if}

<style>
  .modal-backdrop {
    position: fixed;
    top: 0; left: 0; width: 100%; height: 100%;
    background: rgba(0,0,0,0.6);
    backdrop-filter: blur(8px);
    display: flex;
    align-items: center; justify-content: center;
    z-index: 6000;
    transition: all 0.3s;
  }

  .modal {
    background: var(--surface-container-highest);
    width: 90%;
    max-width: 500px;
    border-radius: 32px;
    border: 1px solid var(--border-color);
    box-shadow: 0 40px 100px rgba(0,0,0,0.5);
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .modal-header {
    padding: 1.5rem 2rem;
    border-bottom: 1px solid var(--border-color);
    display: flex; justify-content: space-between; align-items: center;
  }
  .modal-header h2 { margin: 0; font-size: 1.2rem; font-weight: 850; }

  .close-btn { background: none; border: none; color: var(--text-muted); cursor: pointer; }

  .modal-content {
    padding: 2rem;
    display: flex;
    flex-direction: column;
    gap: 2rem;
    max-height: 70vh;
    overflow-y: auto;
  }

  .form-group { display: flex; flex-direction: column; gap: 0.8rem; }
  .form-group label { font-weight: 800; font-size: 0.9rem; color: var(--text-muted); }
  
  input[type="text"], select {
    background: var(--surface-container-high);
    border: 1px solid var(--border-color);
    color: var(--text-main);
    padding: 1rem 1.2rem;
    border-radius: 16px;
    font-size: 1rem;
    font-weight: 700;
    outline: none;
    width: 100%;
    box-sizing: border-box;
  }
  input[type="text"]:focus { border-color: var(--primary); }

  .smart-toggle {
    background: var(--surface-container);
    padding: 1.2rem;
    border-radius: 20px;
    border: 1px solid var(--border-color);
  }

  .switch-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    cursor: pointer;
  }

  .texts { display: flex; flex-direction: column; gap: 0.2rem; }
  .main-label { font-weight: 850; font-size: 1rem; }
  .sub-label { font-size: 0.8rem; color: var(--text-muted); font-weight: 600; }

  .switch-row input {
    width: 48px; height: 24px;
    appearance: none;
    background: var(--surface-container-highest);
    border-radius: 20px;
    position: relative;
    cursor: pointer;
    transition: all 0.3s;
    border: 1px solid var(--border-color);
  }
  .switch-row input:checked { background: var(--primary); border-color: transparent; }
  .switch-row input::after {
    content: "";
    position: absolute;
    top: 2px; left: 2px;
    width: 18px; height: 18px;
    background: white;
    border-radius: 50%;
    transition: all 0.3s;
  }
  .switch-row input:checked::after { transform: translateX(24px); }

  .rules-section {
    display: flex; flex-direction: column; gap: 1.5rem;
    animation: fadeIn 0.3s ease-out;
  }
  .rules-section h3 { margin: 0; font-size: 1rem; font-weight: 850; color: var(--primary); }

  .rules-grid {
    display: flex;
    flex-direction: column;
    gap: 1.2rem;
  }

  .rule { display: flex; flex-direction: column; gap: 0.6rem; }
  .rule label { font-size: 0.8rem; font-weight: 800; color: var(--text-muted); }

  .rating-wrap { display: flex; align-items: center; gap: 1rem; }
  .rating-label { font-size: 0.85rem; font-weight: 700; color: var(--text-main); }

  .radio-group {
    display: flex;
    background: var(--surface-container-high);
    border-radius: 12px;
    padding: 4px;
    border: 1px solid var(--border-color);
  }
  .radio-group button {
    flex: 1;
    padding: 0.6rem;
    border: none;
    background: transparent;
    color: var(--text-muted);
    font-size: 0.85rem;
    font-weight: 800;
    cursor: pointer;
    border-radius: 8px;
    transition: all 0.2s;
  }
  .radio-group button.active { background: var(--primary); color: white; }

  .modal-footer {
    padding: 1.5rem 2rem;
    display: flex; justify-content: flex-end; gap: 1rem;
    border-top: 1px solid var(--border-color);
  }

  .cancel-btn { background: none; border: none; color: var(--text-muted); font-weight: 700; cursor: pointer; padding: 0.8rem; }
  
  .save-btn {
    background: var(--primary);
    color: white;
    padding: 1rem 2rem;
    border-radius: 14px;
    font-weight: 800;
    border: none;
    cursor: pointer;
    transition: all 0.2s;
  }
  .save-btn:hover { transform: translateY(-2px); filter: brightness(1.1); }
  .save-btn:disabled { opacity: 0.5; filter: grayscale(1); }

  @keyframes fadeIn { from { opacity: 0; transform: translateY(-10px); } to { opacity: 1; transform: translateY(0); } }
</style>
