<script>
  import Icon from './Icon.svelte';
  import { fade, fly } from 'svelte/transition';

  let { 
    isOpen = $bindable(false), 
    title = "Confirm Action", 
    message = "Are you sure?", 
    confirmLabel = "Confirm", 
    cancelLabel = "Cancel",
    type = "info", // "info" or "danger"
    icon = null,
    onconfirm,
    oncancel = null
  } = $props();

  function handleConfirm() {
    isOpen = false;
    onconfirm?.();
  }

  function handleCancel() {
    isOpen = false;
    oncancel?.();
  }
</script>

{#if isOpen}
<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="modal-backdrop" transition:fade={{ duration: 200 }} onclick={handleCancel}>
  <div class="modal glass" transition:fly={{ y: 20, duration: 300 }} onclick={(e) => e.stopPropagation()}>
    <div class="icon-indicator" class:danger={type === 'danger'} class:info={type !== 'danger'}>
      <Icon name={icon || (type === 'danger' ? 'trash' : 'info')} size={32} />
    </div>
    
    <h2>{title}</h2>
    <p>{message}</p>

    <div class="actions">
      <button class="btn ghost" onclick={handleCancel}>{cancelLabel}</button>
      <button class="btn" class:danger={type === 'danger'} class:primary={type !== 'danger'} onclick={handleConfirm}>
        {confirmLabel}
      </button>
    </div>
  </div>
</div>
{/if}

<style>
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.7);
    backdrop-filter: blur(12px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 9999;
  }

  .modal {
    background: var(--surface-container-highest);
    padding: 3.5rem 3rem;
    border-radius: 48px;
    width: 90%;
    max-width: 440px;
    text-align: center;
    border: 1px solid var(--border-color);
    box-shadow: 0 40px 100px rgba(0,0,0,0.5);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1.5rem;
  }

  .icon-indicator {
    width: 80px;
    height: 80px;
    border-radius: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: 0.5rem;
  }

  .icon-indicator.danger {
    background: rgba(255, 68, 68, 0.15);
    color: #ff4444;
  }

  .icon-indicator.info {
    background: var(--primary-10);
    color: var(--primary);
  }

  h2 {
    margin: 0;
    font-size: 2rem;
    font-weight: 950;
    letter-spacing: -1px;
    color: white;
    line-height: 1.1;
  }

  p {
    margin: 0;
    font-size: 1.15rem;
    line-height: 1.6;
    color: var(--text-muted);
    font-weight: 600;
  }

  .actions {
    display: flex;
    gap: 1.2rem;
    width: 100%;
    margin-top: 1.5rem;
  }

  .btn {
    flex: 1;
    padding: 1.1rem;
    border-radius: 100px;
    font-weight: 900;
    font-size: 1rem;
    cursor: pointer;
    border: none;
    transition: all 0.25s cubic-bezier(0.2, 0, 0, 1);
  }

  .btn.primary {
    background: var(--primary);
    color: white;
  }

  .btn.danger {
    background: linear-gradient(135deg, #ff4444, #cc0000);
    color: white;
    box-shadow: 0 8px 24px rgba(255, 68, 68, 0.3);
  }

  .btn.ghost {
    background: var(--surface-container-high);
    color: var(--text-main);
  }

  .btn:hover {
    transform: translateY(-3px) scale(1.02);
    filter: brightness(1.1);
  }

  .btn:active {
    transform: translateY(0) scale(0.98);
  }
</style>
