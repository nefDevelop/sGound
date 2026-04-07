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
          <Icon name="xCircle" size={20} color="#f87171" />
        {:else if data.type === 'success'}
          <Icon name="checkCircle" size={20} color="#4ade80" />
        {:else}
          <Icon name="info" size={20} color="#60a5fa" />
        {/if}
      </div>
      <div class="message">{data.message}</div>
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
    gap: 1rem;
    padding: 0.75rem 1.25rem;
    border-radius: 16px;
    background: rgba(30, 41, 59, 0.9);
    border: 1px solid rgba(255, 255, 255, 0.1);
    box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.4);
    min-width: 300px;
    max-width: 450px;
  }

  .icon-section {
    display: flex;
    align-items: center;
  }

  .message {
    flex: 1;
    color: white;
    font-size: 0.95rem;
    font-weight: 500;
  }

  .close {
    background: transparent;
    border: none;
    cursor: pointer;
    padding: 0.2rem;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .toast.error { border-color: rgba(248, 113, 113, 0.4); }
  .toast.success { border-color: rgba(74, 222, 128, 0.4); }
  .toast.info { border-color: rgba(96, 165, 250, 0.4); }
</style>
