<script>
  import { createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';

  const dispatch = createEventDispatcher();
  let username = $state('');
  let password = $state('');
  let loading = $state(false);
  let error = $state('');

  async function handleLogin() {
    if (!username) return;
    loading = true;
    error = '';
    try {
      // 1. Check if user is allowed and if setup is needed
      const statusRes = await fetch(`/api/auth/status?username=${username}`);
      if (!statusRes.ok) {
        if (statusRes.status === 403) error = 'No estás autorizado en este servidor';
        else error = 'Error al comprobar estado del usuario';
        return;
      }
      
      const statusData = await statusRes.json();
      if (statusData.setupRequired) {
        dispatch('setup', { username });
        return;
      }

      // 2. Regular Login
      const res = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password })
      });
      
      if (res.ok) {
        const data = await res.json();
        localStorage.setItem('username', data.username);
        localStorage.setItem('permissions', data.permissions);
        dispatch('login', data);
      } else {
        error = 'Contraseña incorrecta';
      }
    } catch (e) {
      error = 'Error de conexión con el servidor';
    } finally {
      loading = false;
    }
  }
</script>

<div class="login-page">
  <div class="login-card glass">
    <div class="brand">
      <div class="logo"></div>
      <h1>Sgound</h1>
      <p>Entra en tu universo musical</p>
    </div>

    <form onsubmit={(e) => { e.preventDefault(); handleLogin(); }}>
      <div class="input-group">
        <label for="username">Usuario</label>
        <div class="input-wrapper">
          <Icon name="user" size={18} color="var(--text-muted)" />
          <input type="text" id="username" bind:value={username} placeholder="admin" required autofocus />
        </div>
      </div>

      <div class="input-group">
        <label for="password">Contraseña</label>
        <div class="input-wrapper">
          <Icon name="lock" size={18} color="var(--text-muted)" />
          <input type="password" id="password" bind:value={password} placeholder="••••••••" />
        </div>
      </div>

      {#if error}
        <div class="error-msg">{error}</div>
      {/if}

      <button type="submit" class="login-btn" disabled={loading}>
        {loading ? 'Entrando...' : 'Acceder'}
        {#if !loading}<Icon name="logIn" size={18} />{/if}
      </button>
    </form>

    <div class="footer">
      <p>Decentralized Music Management</p>
    </div>
  </div>
</div>

<style>
  .login-page {
    height: 100vh;
    width: 100vw;
    display: flex;
    align-items: center;
    justify-content: center;
    background: radial-gradient(circle at top left, var(--primary-10), transparent),
                radial-gradient(circle at bottom right, rgba(0,0,0,0.5), var(--surface));
    position: fixed;
    top: 0; left: 0; z-index: 1000;
  }

  .login-card {
    width: 100%;
    max-width: 450px;
    padding: 4rem;
    border-radius: 40px;
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid rgba(255, 255, 255, 0.1);
    box-shadow: 0 50px 100px rgba(0,0,0,0.5);
    backdrop-filter: blur(20px);
    animation: slideUp 0.6s cubic-bezier(0.2, 0, 0, 1);
  }

  @keyframes slideUp {
    from { opacity: 0; transform: translateY(30px); }
    to { opacity: 1; transform: translateY(0); }
  }

  .brand { text-align: center; margin-bottom: 3rem; }
  .logo { 
    width: 64px; height: 64px; 
    background: var(--primary); 
    border-radius: 20px; 
    margin: 0 auto 1.5rem;
    box-shadow: 0 10px 30px var(--primary-10);
  }
  h1 { font-size: 2.5rem; font-weight: 950; letter-spacing: -2px; margin: 0; }
  p { color: var(--text-muted); font-weight: 600; margin-top: 0.5rem; }

  form { display: flex; flex-direction: column; gap: 2rem; }

  .input-group { display: flex; flex-direction: column; gap: 0.8rem; }
  .input-group label { font-size: 0.8rem; font-weight: 800; color: var(--text-muted); text-transform: uppercase; letter-spacing: 1px; padding-left: 0.5rem; }
  
  .input-wrapper {
    display: flex;
    align-items: center;
    gap: 1rem;
    background: rgba(0,0,0,0.2);
    padding: 0 1.5rem;
    border-radius: 100px;
    border: 1px solid rgba(255,255,255,0.05);
    transition: all 0.3s;
  }
  .input-wrapper:focus-within { border-color: var(--primary); background: rgba(0,0,0,0.4); }

  input { 
    background: transparent; border: none; color: white; padding: 1.2rem 0; 
    width: 100%; outline: none; font-weight: 600; font-size: 1.1rem;
  }

  .error-msg { 
    color: #f87171; background: rgba(248, 113, 113, 0.1); 
    padding: 1rem; border-radius: 12px; font-weight: 600; font-size: 0.9rem;
    text-align: center;
  }

  .login-btn {
    background: var(--primary);
    color: white;
    border: none;
    padding: 1.2rem;
    border-radius: 100px;
    font-weight: 900;
    font-size: 1.1rem;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 1rem;
    box-shadow: 0 15px 35px var(--primary-10);
    transition: all 0.3s cubic-bezier(0.2, 0, 0, 1);
    margin-top: 1rem;
  }
  .login-btn:hover { transform: translateY(-3px); filter: brightness(1.1); box-shadow: 0 20px 45px var(--primary-10); }
  .login-btn:disabled { opacity: 0.5; cursor: not-allowed; transform: none; }

  .footer { margin-top: 3rem; text-align: center; opacity: 0.3; font-size: 0.8rem; font-weight: 800; text-transform: uppercase; letter-spacing: 2px; }
</style>
