<script>
  import { createEventDispatcher } from 'svelte';
  import { fade } from 'svelte/transition';
  import Icon from './Icon.svelte';

  const dispatch = createEventDispatcher();
  let username = $state('');
  let password = $state('');
  let showPassword = $state(false);
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
  <div class="background-blobs">
    <div class="blob blob-1"></div>
    <div class="blob blob-2"></div>
    <div class="blob blob-3"></div>
  </div>

  <div class="login-card">
    <div class="brand">
      <div class="logo-outer">
        <div class="logo">
          <div class="logo-icon"></div>
        </div>
      </div>
      <h1>Sgound</h1>
      <p class="subtitle">Personal Music Cloud</p>
    </div>

    <form onsubmit={(e) => { e.preventDefault(); handleLogin(); }}>
      <div class="input-group">
        <label for="username">Usuario</label>
        <div class="input-wrapper" class:error={error && !username}>
          <Icon name="user" size={20} color="var(--primary)" />
          <input type="text" id="username" bind:value={username} placeholder="admin" required autocomplete="username" />
        </div>
      </div>

      <div class="input-group">
        <label for="password">Contraseña</label>
        <div class="input-wrapper" class:error={error && username}>
          <Icon name="lock" size={20} color="var(--primary)" />
          <input type={showPassword ? "text" : "password"} id="password" bind:value={password} placeholder="••••••••" autocomplete="current-password" />
          <button type="button" class="visibility-toggle" onclick={() => showPassword = !showPassword} tabindex="-1">
            <Icon name={showPassword ? "eyeOff" : "eye"} size={18} color="var(--text-muted)" />
          </button>
        </div>
      </div>

      {#if error}
        <div class="error-msg" in:fade>
          <Icon name="alertCircle" size={16} />
          {error}
        </div>
      {/if}

      <button type="submit" class="login-btn" disabled={loading}>
        {#if loading}
          <div class="loading-spinner"></div>
          <span>Entrando...</span>
        {:else}
          <span>Comenzar Sesión</span>
          <Icon name="arrowRight" size={20} />
        {/if}
      </button>
    </form>

    <div class="footer">
      <span>SGOUND</span>
      <div class="dots"></div>
      <span>V1.0</span>
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
    background-color: #05070a;
    position: fixed;
    top: 0; left: 0; z-index: 1000;
    overflow: hidden;
    color: white;
    font-family: var(--font-body);
  }

  .background-blobs {
    position: absolute;
    width: 100%;
    height: 100%;
    overflow: hidden;
    z-index: 1;
    opacity: 0.6;
    filter: blur(80px);
  }

  .blob {
    position: absolute;
    border-radius: 50%;
    animation: blobify 20s infinite alternate;
  }

  .blob-1 {
    width: 600px;
    height: 600px;
    background: radial-gradient(circle, var(--primary) 0%, transparent 70%);
    top: -100px;
    left: -100px;
  }

  .blob-2 {
    width: 500px;
    height: 500px;
    background: radial-gradient(circle, var(--tertiary) 0%, transparent 70%);
    bottom: -100px;
    right: -100px;
    animation-delay: -5s;
  }

  .blob-3 {
    width: 400px;
    height: 400px;
    background: radial-gradient(circle, var(--secondary) 0%, transparent 70%);
    top: 40%;
    left: 40%;
    animation-delay: -10s;
  }

  @keyframes blobify {
    0% { transform: translate(0, 0) scale(1); }
    33% { transform: translate(30px, -50px) scale(1.1); }
    66% { transform: translate(-20px, 20px) scale(0.9); }
    100% { transform: translate(0, 0) scale(1); }
  }

  .login-card {
    position: relative;
    z-index: 10;
    width: 100%;
    max-width: 440px;
    padding: 3rem;
    border-radius: 40px;
    background: rgba(18, 20, 26, 0.7);
    border: 1px solid rgba(255, 255, 255, 0.08);
    box-shadow: 0 30px 60px rgba(0,0,0,0.6);
    backdrop-filter: blur(40px) saturate(180%);
    animation: cardEntrance 0.8s cubic-bezier(0.2, 1, 0.3, 1);
  }

  @keyframes cardEntrance {
    from { opacity: 0; transform: translateY(40px) scale(0.95); }
    to { opacity: 1; transform: translateY(0) scale(1); }
  }

  .brand {
    text-align: center;
    margin-bottom: 3.5rem;
  }

  .logo-outer {
    width: 80px;
    height: 80px;
    margin: 0 auto 1.5rem;
    position: relative;
  }

  .logo-outer::after {
    content: '';
    position: absolute;
    inset: -10px;
    background: var(--primary);
    filter: blur(25px);
    opacity: 0.3;
    border-radius: 30px;
    z-index: -1;
  }

  .logo { 
    width: 100%;
    height: 100%;
    background: linear-gradient(135deg, var(--primary), var(--tertiary));
    border-radius: 24px; 
    display: flex;
    align-items: center;
    justify-content: center;
    color: white;
    box-shadow: inset 0 2px 4px rgba(255,255,255,0.2), 0 10px 20px rgba(0,0,0,0.3);
  }

  .logo-icon {
    width: 48px;
    height: 48px;
    background-color: currentColor;
    -webkit-mask: url('/icons.svg?v=3') no-repeat center / contain;
    mask: url('/icons.svg?v=3') no-repeat center / contain;
  }

  h1 { 
    font-family: var(--font-headline);
    font-size: 3rem; 
    font-weight: 900; 
    letter-spacing: -1.5px; 
    margin: 0; 
    background: linear-gradient(to bottom, #fff, #aaa);
    -webkit-background-clip: text;
    background-clip: text;
    -webkit-text-fill-color: transparent;
  }

  .subtitle { 
    color: var(--text-muted); 
    font-weight: 700; 
    margin-top: 0.2rem; 
    text-transform: uppercase;
    font-size: 0.75rem;
    letter-spacing: 4px;
    opacity: 0.8;
  }

  form { display: flex; flex-direction: column; gap: 1.8rem; }

  .input-group { display: flex; flex-direction: column; gap: 0.7rem; }
  .input-group label { 
    font-size: 0.75rem; 
    font-weight: 800; 
    color: var(--text-muted); 
    text-transform: uppercase; 
    letter-spacing: 1.5px; 
    padding-left: 0.5rem; 
  }
  
  .input-wrapper {
    display: flex;
    align-items: center;
    gap: 1.2rem;
    background: rgba(0,0,0,0.3);
    padding: 0 1.5rem;
    border-radius: 18px;
    border: 1px solid rgba(255,255,255,0.06);
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .input-wrapper:focus-within { 
    border-color: var(--primary); 
    background: rgba(0,0,0,0.5); 
    box-shadow: 0 0 0 4px rgba(var(--primary-rgb), 0.1);
    transform: translateY(-2px);
  }

  .input-wrapper.error {
    border-color: #f87171;
  }

  input { 
    background: transparent; 
    border: none; 
    color: white; 
    padding: 1.1rem 0; 
    width: 100%; 
    outline: none; 
    font-weight: 600; 
    font-size: 1.05rem;
  }

  input::placeholder { color: rgba(255,255,255,0.2); }

  .visibility-toggle {
    background: none;
    border: none;
    padding: 0.5rem;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0.5;
    transition: all 0.2s;
    border-radius: 10px;
  }
  .visibility-toggle:hover {
    opacity: 1;
    background: rgba(255,255,255,0.05);
  }

  .error-msg { 
    color: #fca5a5; 
    background: rgba(248, 113, 113, 0.1); 
    padding: 0.75rem 1rem; 
    border-radius: 14px; 
    font-weight: 700; 
    font-size: 0.85rem;
    display: flex;
    align-items: center;
    gap: 0.6rem;
    border: 1px solid rgba(248, 113, 113, 0.1);
  }

  .login-btn {
    background: var(--primary);
    color: white;
    border: none;
    padding: 1.1rem;
    border-radius: 18px;
    font-weight: 800;
    font-size: 1.05rem;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 1rem;
    transition: all 0.4s cubic-bezier(0.2, 1, 0.3, 1);
    margin-top: 0.5rem;
    position: relative;
    overflow: hidden;
  }

  .login-btn::before {
    content: '';
    position: absolute;
    top: 0; left: -100%;
    width: 100%; height: 100%;
    background: linear-gradient(90deg, transparent, rgba(255,255,255,0.2), transparent);
    transition: all 0.6s;
  }

  .login-btn:hover { 
    transform: translateY(-4px); 
    filter: brightness(1.1); 
    box-shadow: 0 12px 30px rgba(var(--primary-rgb), 0.3); 
  }

  .login-btn:hover::before {
    left: 100%;
  }

  .login-btn:active { transform: translateY(-1px); }
  
  .login-btn:disabled { 
    opacity: 0.6; 
    cursor: not-allowed; 
    transform: none !important; 
    box-shadow: none !important;
  }

  .loading-spinner {
    width: 20px;
    height: 20px;
    border: 3px solid rgba(255,255,255,0.3);
    border-top-color: white;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .footer { 
    margin-top: 3.5rem; 
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 1.5rem;
    opacity: 0.2; 
    font-size: 0.7rem; 
    font-weight: 900; 
    letter-spacing: 2px; 
    transition: opacity 0.3s;
  }

  .footer:hover { opacity: 0.5; }

  .dots {
    width: 4px;
    height: 4px;
    background: white;
    border-radius: 50%;
  }

  @media (max-width: 480px) {
    .login-card {
      padding: 2rem;
      max-width: 90%;
      border-radius: 32px;
    }
    h1 { font-size: 2.2rem; }
  }
</style>
