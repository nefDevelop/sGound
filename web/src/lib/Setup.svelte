<script>
  import { createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';

  const dispatch = createEventDispatcher();
  let { username } = $props();
  let password = $state('');
  let confirmPassword = $state('');
  let loading = $state(false);
  let error = $state('');

  async function handleSetup() {
    if (password !== confirmPassword) {
      error = 'Las contraseñas no coinciden';
      return;
    }
    loading = true;
    error = '';
    try {
      const res = await fetch('/api/auth/setup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password })
      });
      if (res.ok) {
        const data = await res.json();
        localStorage.setItem('username', data.username);
        localStorage.setItem('permissions', data.permissions);
        dispatch('complete');
      } else {
        const msg = await res.text();
        error = msg || 'Error en el setup';
      }
    } catch (e) {
      error = 'Error de conexión';
    } finally {
      loading = false;
    }
  }
</script>

<div class="setup-page">
  <div class="setup-card glass">
    <div class="brand">
      <div class="logo"></div>
      <h1>Hola {username}</h1>
      <p>Configura tu nueva contraseña para comenzar</p>
    </div>

    <form onsubmit={(e) => { e.preventDefault(); handleSetup(); }}>

      <div class="input-group">
        <label for="password">Contraseña</label>
        <div class="input-wrapper">
          <Icon name="lock" size={18} color="var(--text-muted)" />
          <input type="password" id="password" bind:value={password} placeholder="Mínimo 8 caracteres" required autofocus />
        </div>
      </div>

      <div class="input-group">
        <label for="confirm">Confirmar Contraseña</label>
        <div class="input-wrapper">
          <Icon name="shield" size={18} color="var(--text-muted)" />
          <input type="password" id="confirm" bind:value={confirmPassword} required />
        </div>
      </div>

      {#if error}
        <div class="error-msg">{error}</div>
      {/if}

      <button type="submit" class="setup-btn" disabled={loading}>
        {loading ? 'Inicializando...' : 'Crear Cuenta y Comenzar'}
        {#if !loading}<Icon name="check" size={18} />{/if}
      </button>
    </form>
  </div>
</div>

<style>
  .setup-page {
    height: 100vh;
    width: 100vw;
    display: flex;
    align-items: center;
    justify-content: center;
    background: radial-gradient(circle at top left, var(--primary-10), transparent),
                radial-gradient(circle at bottom right, rgba(0,0,0,0.8), var(--surface));
    position: fixed;
    top: 0; left: 0; z-index: 2000;
    font-family: var(--font-main);
  }

  .setup-card {
    width: 100%;
    max-width: 500px;
    padding: 4rem;
    border-radius: 48px;
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid rgba(255, 255, 255, 0.1);
    box-shadow: 0 60px 120px rgba(0,0,0,0.6);
    backdrop-filter: blur(30px);
    animation: scaleIn 0.8s cubic-bezier(0.2, 0, 0, 1);
  }

  @keyframes scaleIn {
    from { opacity: 0; transform: scale(0.9) translateY(20px); }
    to { opacity: 1; transform: scale(1) translateY(0); }
  }

  .brand { text-align: center; margin-bottom: 3rem; }
  .logo { transform: rotate(-10deg); width: 80px; height: 80px; background: var(--primary); border-radius: 24px; margin: 0 auto 2rem; box-shadow: 0 15px 40px var(--primary-10); }
  
  h1 { font-size: 2.2rem; font-weight: 900; letter-spacing: -1.5px; margin: 0; color: white; }
  p { color: var(--text-muted); font-weight: 600; margin-top: 0.8rem; font-size: 1.1rem; }

  form { display: flex; flex-direction: column; gap: 1.8rem; }

  .input-group { display: flex; flex-direction: column; gap: 0.8rem; }
  .input-group label { font-size: 0.75rem; font-weight: 800; color: var(--text-muted); text-transform: uppercase; letter-spacing: 2px; padding-left: 0.5rem; }
  
  .input-wrapper {
    display: flex;
    align-items: center;
    gap: 1rem;
    background: rgba(255,255,255,0.03);
    padding: 0 1.5rem;
    border-radius: 20px;
    border: 1px solid rgba(255,255,255,0.05);
    transition: all 0.3s cubic-bezier(0.2, 0, 0, 1);
  }
  .input-wrapper:focus-within { border-color: var(--primary); background: rgba(0,0,0,0.5); transform: translateX(5px); }

  input { 
    background: transparent; border: none; color: white; padding: 1.2rem 0; 
    width: 100%; outline: none; font-weight: 600; font-size: 1.1rem;
  }

  .error-msg { 
    color: #fca5a5; background: rgba(239, 68, 68, 0.1); 
    padding: 1rem; border-radius: 16px; font-weight: 700; font-size: 0.9rem;
    text-align: center; border: 1px solid rgba(239,68,68,0.2);
  }

  .setup-btn {
    background: var(--primary);
    color: white;
    border: none;
    padding: 1.4rem;
    border-radius: 20px;
    font-weight: 950;
    font-size: 1.1rem;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 1.2rem;
    box-shadow: 0 20px 40px var(--primary-10);
    transition: all 0.4s cubic-bezier(0.2, 0, 0, 1);
    margin-top: 1rem;
    text-transform: uppercase;
    letter-spacing: 1px;
  }
  .setup-btn:hover { transform: translateY(-5px) scale(1.02); filter: brightness(1.1); box-shadow: 0 25px 55px var(--primary-10); }
  .setup-btn:active { transform: scale(0.98); }
  .setup-btn:disabled { opacity: 0.4; cursor: not-allowed; transform: none; }
</style>
