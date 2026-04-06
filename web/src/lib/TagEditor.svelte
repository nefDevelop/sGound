<script>
  import { createEventDispatcher } from 'svelte';
  
  export let isOpen = false;
  export let track = { id: '', title: '', artist: '', album: '' };
  
  const dispatch = createEventDispatcher();
  let loading = false;
  let saveSuccess = false;

  async function handleSave() {
    loading = true;
    try {
      const res = await fetch(`/api/tracks/${track.id}/tags`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          title: track.title,
          artist: track.artist,
          album: track.album
        })
      });
      
      if (!res.ok) throw new Error("Failed to update tags");
      
      saveSuccess = true;
      setTimeout(() => {
        saveSuccess = false;
        closeModal();
      }, 1500);
      
    } catch (e) {
      console.error(e);
      alert("Error saving tags: " + e.message);
    } finally {
      loading = false;
    }
  }

  function closeModal() {
    isOpen = false;
    dispatch('close');
  }
</script>

{#if isOpen}
<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="modal-backdrop" on:click={closeModal}>
  <div class="modal glass" on:click|stopPropagation>
    <div class="modal-header">
      <h2>Edit Track Tags</h2>
      <button class="close-btn" on:click={closeModal}>✕</button>
    </div>

    <form class="tag-form" on:submit|preventDefault={handleSave}>
      <label>
        <span>Title</span>
        <input type="text" bind:value={track.title} placeholder="Enter title..." required />
      </label>

      <label>
        <span>Artist</span>
        <input type="text" bind:value={track.artist} placeholder="Enter artist..." required />
      </label>

      <label>
        <span>Album</span>
        <input type="text" bind:value={track.album} placeholder="Enter album..." required />
      </label>

      <div class="actions">
        <button type="button" class="cancel-btn" on:click={closeModal}>Cancel</button>
        <button type="submit" class="save-btn" disabled={loading}>
          {loading ? 'Saving...' : saveSuccess ? 'Saved ✓' : 'Save Tags'}
        </button>
      </div>
    </form>
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
    align-items: center;
    justify-content: center;
    z-index: 100;
  }

  .modal {
    background: var(--surface-color);
    padding: 2rem;
    border-radius: 20px;
    width: 90%;
    max-width: 450px;
    box-shadow: 0 25px 50px -12px rgba(0,0,0,0.5);
    animation: popIn 0.3s cubic-bezier(0.175, 0.885, 0.32, 1.275);
  }

  @keyframes popIn {
    0% { opacity: 0; transform: scale(0.9) translateY(20px); }
    100% { opacity: 1; transform: scale(1) translateY(0); }
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 2rem;
  }

  .modal-header h2 {
    margin: 0;
    color: var(--text-main);
  }

  .close-btn {
    font-size: 1.2rem;
    color: var(--text-muted);
  }

  .tag-form {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }

  .tag-form label {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    color: var(--text-muted);
  }

  .tag-form input {
    background: rgba(0,0,0,0.2);
    border: 1px solid var(--border-color);
    color: var(--text-main);
    padding: 0.8rem 1rem;
    border-radius: 8px;
    font-family: inherit;
    font-size: 1rem;
    transition: border-color 0.2s;
  }

  .tag-form input:focus {
    outline: none;
    border-color: var(--accent-color);
    box-shadow: 0 0 0 2px var(--accent-glow);
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 1rem;
    margin-top: 1rem;
  }

  .cancel-btn {
    padding: 0.8rem 1.5rem;
    border-radius: 8px;
    color: var(--text-muted);
  }

  .cancel-btn:hover {
    color: var(--text-main);
    background: rgba(255,255,255,0.05);
  }

  .save-btn {
    background: var(--accent-color);
    color: white;
    padding: 0.8rem 2rem;
    border-radius: 8px;
    font-weight: 600;
  }

  .save-btn:hover:not(:disabled) {
    transform: translateY(-2px);
    box-shadow: 0 4px 15px var(--accent-glow);
  }

  .save-btn:disabled {
    opacity: 0.7;
    cursor: wait;
  }
</style>
