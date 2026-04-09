<script>
  import { fade, fly } from 'svelte/transition';
  import Icon from './Icon.svelte';

  let { 
    isOpen = $bindable(false), 
    title = "", 
    children, 
    width = "500px",
    showClose = true
  } = $props();

  function close() {
    isOpen = false;
  }

  function handleKeydown(e) {
    if (e.key === 'Escape' && isOpen) close();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if isOpen}
<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="modal-backdrop" transition:fade={{ duration: 200 }} onclick={close}>
  <div 
    class="modal glass" 
    transition:fly={{ y: 30, duration: 400, opacity: 0 }} 
    onclick={(e) => e.stopPropagation()}
    style="--modal-width: {width};"
  >
    {#if title || showClose}
      <div class="modal-header">
        {#if title}
          <h2>{title}</h2>
        {/if}
        {#if showClose}
          <button class="close-button" onclick={close} aria-label="Close">
            <Icon name="x" size={24} />
          </button>
        {/if}
      </div>
    {/if}

    <div class="modal-content">
      {@render children()}
    </div>
  </div>
</div>
{/if}

<style>
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.4);
    backdrop-filter: blur(20px) saturate(180%);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 10000;
    padding: 20px;
  }

  .modal {
    background: var(--surface-container-highest);
    width: 100%;
    max-width: var(--modal-width);
    max-height: 90vh;
    border-radius: 40px;
    border: 1px solid rgba(255, 255, 255, 0.1);
    box-shadow: 0 50px 100px -20px rgba(0, 0, 0, 0.5);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    position: relative;
  }

  .modal::before {
    content: '';
    position: absolute;
    inset: 0;
    background: radial-gradient(circle at top left, var(--primary-20), transparent 40%);
    pointer-events: none;
    opacity: 0.5;
  }

  .modal-header {
    padding: 2rem 2.5rem 1rem;
    display: flex;
    align-items: center;
    justify-content: space-between;
    position: relative;
    z-index: 1;
  }

  h2 {
    margin: 0;
    font-size: 1.8rem;
    font-weight: 900;
    color: white;
    letter-spacing: -0.5px;
  }

  .close-button {
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    color: var(--text-muted);
    width: 44px;
    height: 44px;
    border-radius: 14px;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.2s;
  }

  .close-button:hover {
    background: rgba(255, 255, 255, 0.1);
    color: white;
    transform: scale(1.05);
  }

  .modal-content {
    padding: 0 2.5rem 2.5rem;
    overflow-y: auto;
    flex: 1;
    position: relative;
    z-index: 1;
    scrollbar-width: thin;
    scrollbar-color: var(--primary-20) transparent;
  }

  .modal-content::-webkit-scrollbar {
    width: 6px;
  }
  .modal-content::-webkit-scrollbar-thumb {
    background: var(--primary-20);
    border-radius: 10px;
  }

  @media (max-width: 600px) {
    .modal {
      border-radius: 32px;
    }
    .modal-header {
      padding: 1.5rem 1.5rem 1rem;
    }
    .modal-content {
      padding: 0 1.5rem 1.5rem;
    }
  }
</style>
