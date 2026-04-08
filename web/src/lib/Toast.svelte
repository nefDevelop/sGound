<script>
  import { toast } from './stores.js';
  import { fade, fly } from 'svelte/transition';
  import Icon from './Icon.svelte';

  $: data = $toast;
</script>

{#if data}
  <div class="toast-wrapper" in:fly={{ y: 20, duration: 300 }} out:fade>
    <div class="toast glass {data.type}">
      <div class="icon-section">
        {#if data.type === 'error'}
          <Icon name="xCircle" size={20} color="var(--error, #f87171)" />
        {:else if data.type === 'success'}
          <Icon name="checkCircle" size={20} color="var(--primary)" />
        {:else}
          <Icon name="info" size={20} color="var(--secondary)" />
        {/if}
      </div>
      <div class="message">{@html data.message}</div>
      <button class="close" on:click={() => toast.set(null)}>
        <Icon name="x" size={14} color="rgba(255,255,255,0.4)" />
      </button>
    </div>
  </div>
{/if}

<style>
  .toast-wrapper {
    position: fixed;
    bottom: 110px; /* Above player */
    left: 50%;
    transform: translateX(-50%);
    z-index: 1000;
    pointer-events: none;
  }

  .toast {
    pointer-events: auto;
    display: flex;
    align-items: center;
    gap: 1.2rem;
    padding: 1rem 1.5rem;
    border-radius: 24px;
    background: var(--surface-container-highest);
    border: 1px solid var(--border-color);
    box-shadow: 0 20px 40px rgba(0, 0, 0, 0.4);
    min-width: 320px;
    max-width: 500px;
    backdrop-filter: blur(20px);
  }

  .icon-section {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    background: rgba(255,255,255,0.03);
    border-radius: 12px;
  }

  .message {
    flex: 1;
    color: var(--text-main);
    font-size: 0.92rem;
    font-weight: 700;
    line-height: 1.4;
  }
  
  .message :global(b) {
    color: var(--primary);
    font-weight: 950;
  }

  .close {
    background: transparent;
    border: none;
    cursor: pointer;
    padding: 0.5rem;
    border-radius: 10px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background 0.2s;
  }
  .close:hover { background: rgba(255,255,255,0.05); }

  .toast.error { border-color: var(--error-30); }
  .toast.success { border-color: var(--primary-30); }
  .toast.info { border-color: var(--secondary-30); }
</style>
