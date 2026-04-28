<script>
  import { onMount, onDestroy } from 'svelte';
  import Icon from './Icon.svelte';

  let logs = $state([]);
  let autoScroll = $state(true);
  let scrollContainer = $state(null);
  let eventSource;

  onMount(() => {
    connectStream();
  });

  onDestroy(() => {
    if (eventSource) eventSource.close();
  });

  function connectStream() {
    if (eventSource) eventSource.close();
    
    // Note: EventSource doesn't support custom headers
    eventSource = new EventSource('/api/logs?stream=true'); 

    eventSource.onmessage = (event) => {
      try {
        const entry = JSON.parse(event.data);
        logs = [...logs, entry];
        if (logs.length > 200) logs = logs.slice(1);
        if (autoScroll) scrollToBottom();
      } catch (e) {
        console.error("Failed to parse log entry", e);
      }
    };

    eventSource.onerror = () => {
      console.warn("Log stream disconnected, retrying...");
      eventSource.close();
      setTimeout(connectStream, 3000);
    };
  }

  function scrollToBottom() {
    setTimeout(() => {
      if (scrollContainer) {
        scrollContainer.scrollTop = scrollContainer.scrollHeight;
      }
    }, 50);
  }

  function formatTime(ts) {
    const d = new Date(ts);
    return d.toLocaleTimeString();
  }

  function getLevelColor(level) {
    switch (level?.toUpperCase()) {
      case 'ERROR': return '#ff4444';
      case 'WARN': return '#ffbb33';
      case 'DEBUG': return '#00C851';
      default: return '#33b5e5';
    }
  }
</script>

<div class="log-viewer">
  <div class="log-header">
    <div class="title">
      <Icon name="terminal" size={18} />
      <span>Server Logs</span>
    </div>
    <div class="controls">
      <label class="auto-scroll">
        <input type="checkbox" bind:checked={autoScroll} />
        Auto-scroll
      </label>
      <button class="icon-btn" onclick={connectStream} title="Reconnect stream">
        <Icon name="refreshCw" size={14} />
      </button>
    </div>
  </div>

  <div class="log-content" bind:this={scrollContainer}>
    {#if logs.length === 0}
      <div class="empty">No logs available yet...</div>
    {:else}
      {#each logs as log}
        <div class="log-entry">
          <span class="time">[{formatTime(log.timestamp)}]</span>
          <span class="level" style="color: {getLevelColor(log.level)}">{log.level}</span>
          <span class="msg">{log.message}</span>
          {#if log.fields && Object.keys(log.fields).length > 0}
            <span class="fields">
              {#each Object.entries(log.fields) as [k, v]}
                <span class="field"><span class="key">{k}:</span> {v}</span>
              {/each}
            </span>
          {/if}
        </div>
      {/each}
    {/if}
  </div>
</div>

<style>
  .log-viewer {
    background: #0d1117;
    border-radius: 24px;
    border: 1px solid var(--border-color);
    display: flex;
    flex-direction: column;
    height: 600px;
    overflow: hidden;
    font-family: 'JetBrains Mono', 'Fira Code', monospace;
  }

  .log-header {
    padding: 1rem 1.5rem;
    background: rgba(255,255,255,0.03);
    border-bottom: 1px solid rgba(255,255,255,0.05);
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .log-header .title { display: flex; align-items: center; gap: 0.8rem; font-weight: 800; font-size: 0.9rem; color: var(--text-muted); }

  .controls { display: flex; align-items: center; gap: 1rem; }
  .auto-scroll { font-size: 0.75rem; color: var(--text-muted); display: flex; align-items: center; gap: 0.5rem; cursor: pointer; }
  .auto-scroll input { cursor: pointer; }

  .log-content {
    flex: 1;
    overflow-y: auto;
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
    scrollbar-width: thin;
  }

  .log-entry { font-size: 0.8rem; line-height: 1.4; white-space: pre-wrap; word-break: break-all; }
  
  .time { color: #6a737d; margin-right: 0.5rem; }
  .level { font-weight: 900; margin-right: 0.5rem; min-width: 50px; display: inline-block; }
  .msg { color: #e1e4e8; }
  
  .fields { margin-left: 1rem; color: #8b949e; display: inline-flex; flex-wrap: wrap; gap: 0.8rem; }
  .field { border-bottom: 1px solid rgba(255,255,255,0.05); }
  .key { color: var(--primary); font-weight: 700; }

  .empty { padding: 4rem; text-align: center; color: var(--text-muted); font-size: 0.9rem; }

  .icon-btn { 
    background: transparent; 
    border: none; 
    color: var(--text-muted); 
    cursor: pointer; 
    padding: 4px;
    border-radius: 6px;
  }
  .icon-btn:hover { background: rgba(255,255,255,0.05); color: white; }
</style>
