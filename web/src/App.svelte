<script>
  import { onMount } from 'svelte';
  import { getState, getStats, uploadBackup } from './lib/api.js';
  import Dashboard from './components/Dashboard.svelte';
  import Generator from './components/Generator.svelte';
  import { num } from './lib/format.js';

  let tab = $state('dashboard');
  let state = $state(null);
  let stats = $state(null);
  let loading = $state(true);
  let uploading = $state(false);
  let error = $state('');
  let fileInput;

  async function load() {
    loading = true;
    error = '';
    try {
      [state, stats] = await Promise.all([getState(), getStats()]);
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  onMount(load);

  async function onFile(e) {
    const file = e.target.files?.[0];
    if (!file) return;
    uploading = true;
    error = '';
    try {
      await uploadBackup(file);
      await load();
    } catch (err) {
      error = 'Upload failed: ' + err.message;
    } finally {
      uploading = false;
      if (fileInput) fileInput.value = '';
    }
  }
</script>

<div class="app">
  <header class="topbar">
    <div class="container bar">
      <div class="brand">
        <div class="logo">♫</div>
        <div>
          <div class="name">go_muse</div>
          <div class="tagline faint">listening intelligence</div>
        </div>
      </div>

      <nav class="tabs">
        <button class:active={tab === 'dashboard'} onclick={() => (tab = 'dashboard')}>Dashboard</button>
        <button class:active={tab === 'generate'} onclick={() => (tab = 'generate')}>Generate</button>
      </nav>

      <div class="right">
        {#if state}
          <span class="pill">
            {num(state.totalTracks)} tracks
            {#if state.hasBackup}· {num(state.matched)} matched{/if}
          </span>
          <span class="pill" class:good={state.hasBackup}>
            {state.hasBackup ? '● backup loaded' : '○ no backup'}
          </span>
        {/if}
        <button class="btn btn-primary" onclick={() => fileInput.click()} disabled={uploading}>
          {uploading ? 'Parsing…' : 'Upload .pxpl'}
        </button>
        <input bind:this={fileInput} type="file" accept=".pxpl" onchange={onFile} hidden />
      </div>
    </div>
  </header>

  <main class="container">
    {#if error}
      <div class="banner error">{error}</div>
    {/if}

    {#if loading}
      <div class="center">Loading library…</div>
    {:else if !state}
      <div class="center">Could not reach the server.</div>
    {:else if !state.hasDb && !state.hasBackup}
      <div class="onboard card">
        <h2>No data yet</h2>
        <p class="muted">
          Point the server at a <code>gomuse.db</code> (run the analyzer first) for audio attributes, then
          upload your <code>.pxpl</code> backup here to layer in listening history. The recommender combines both.
        </p>
        <button class="btn btn-primary" onclick={() => fileInput.click()}>Upload .pxpl backup</button>
      </div>
    {:else if tab === 'dashboard'}
      {#if stats}<Dashboard {stats} hasBackup={state.hasBackup} />{/if}
    {:else}
      <Generator appState={state} />
    {/if}
  </main>

  <footer class="container faint">
    go_muse · analysis × listening data → playlists. Charts by FlareCharts.
  </footer>
</div>

<style>
  .app { min-height: 100vh; display: flex; flex-direction: column; }
  .topbar { position: sticky; top: 0; z-index: 50; backdrop-filter: blur(14px);
    background: color-mix(in srgb, var(--bg) 78%, transparent); border-bottom: 1px solid var(--border); }
  .bar { display: flex; align-items: center; gap: 20px; height: 66px; }

  .brand { display: flex; align-items: center; gap: 11px; }
  .logo { width: 38px; height: 38px; border-radius: 11px; display: grid; place-items: center; font-size: 19px;
    background: linear-gradient(145deg, #9a8dff, #36d6c3); color: #0b0a16; box-shadow: 0 6px 18px -6px #7c6cff88; }
  .name { font-weight: 700; font-size: 17px; letter-spacing: -0.02em; }
  .tagline { font-size: 11.5px; letter-spacing: 0.04em; text-transform: uppercase; }

  .tabs { display: flex; gap: 4px; margin-left: 8px; background: var(--surface); padding: 4px; border-radius: 12px; border: 1px solid var(--border); }
  .tabs button { padding: 7px 16px; border-radius: 9px; color: var(--muted); font-weight: 550; }
  .tabs button.active { background: var(--surface-3); color: var(--text); box-shadow: var(--shadow); }

  .right { margin-left: auto; display: flex; align-items: center; gap: 10px; }
  .pill.good { color: var(--accent-2); border-color: color-mix(in srgb, var(--accent-2) 40%, transparent); }

  main { flex: 1; padding: 26px 24px 40px; width: 100%; }
  footer { padding: 18px 24px 30px; font-size: 12.5px; }

  .banner { padding: 11px 16px; border-radius: 11px; margin-bottom: 18px; }
  .banner.error { background: color-mix(in srgb, var(--danger) 16%, transparent); border: 1px solid color-mix(in srgb, var(--danger) 40%, transparent); color: #ffc0c0; }

  .center { text-align: center; color: var(--muted); padding: 80px 0; }
  .onboard { max-width: 560px; margin: 60px auto; padding: 32px; text-align: center; }
  .onboard h2 { margin-bottom: 10px; }
  .onboard p { margin: 0 0 20px; }
  code { background: var(--surface-3); padding: 1px 6px; border-radius: 5px; font-size: 13px; }

  @media (max-width: 720px) {
    .tagline, .pill { display: none; }
    .bar { gap: 12px; }
  }
</style>
