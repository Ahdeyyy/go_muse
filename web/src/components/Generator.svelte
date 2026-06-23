<script>
  import Card from './Card.svelte';
  import { generatePlaylist, downloadM3U } from '../lib/api.js';
  import { titleCase, pct } from '../lib/format.js';

  let { appState } = $props();

  // ---- form state ----
  let mood = $state('');
  let activity = $state('');
  let era = $state('any');
  let useEnergy = $state(false);
  let energy = $state(60); // 0..100
  let discovery = $state(50); // 0..100 (familiar -> new)
  let minSongs = $state(10);
  let maxSongs = $state(25);
  let favoritesOnly = $state(false);
  let strictEra = $state(false);
  let selectedGenres = $state([]);
  let selectedArtists = $state([]);
  let artistQuery = $state('');
  let playlistName = $state('');

  let result = $state(null);
  let loading = $state(false);
  let error = $state('');
  let seed = $state(0);

  const artistMatches = $derived(
    artistQuery.trim().length < 2
      ? []
      : (appState.artists || [])
          .filter((a) => a.toLowerCase().includes(artistQuery.toLowerCase()) && !selectedArtists.includes(a))
          .slice(0, 8)
  );

  function toggleGenre(g) {
    selectedGenres = selectedGenres.includes(g)
      ? selectedGenres.filter((x) => x !== g)
      : [...selectedGenres, g];
  }
  function addArtist(a) {
    if (!selectedArtists.includes(a)) selectedArtists = [...selectedArtists, a];
    artistQuery = '';
  }
  function removeArtist(a) {
    selectedArtists = selectedArtists.filter((x) => x !== a);
  }

  function buildRequest() {
    return {
      mood: mood || '',
      activity: activity || '',
      era: era === 'any' ? '' : era,
      energy: useEnergy ? energy / 100 : null,
      discovery: discovery / 100,
      minSongs: Number(minSongs) || 0,
      maxSongs: Number(maxSongs) || 25,
      genres: selectedGenres,
      artists: selectedArtists,
      favoritesOnly,
      strictEra,
      seed,
    };
  }

  async function generate(reshuffle = false) {
    error = '';
    loading = true;
    seed = reshuffle || seed === 0 ? Math.floor(Math.random() * 1e9) : seed;
    try {
      result = await generatePlaylist(buildRequest());
    } catch (e) {
      error = e.message;
      result = null;
    } finally {
      loading = false;
    }
  }

  function defaultName() {
    const parts = [mood, activity].filter(Boolean).map(titleCase);
    return parts.length ? parts.join(' ') : 'go_muse mix';
  }

  async function download() {
    try {
      await downloadM3U(buildRequest(), playlistName.trim() || defaultName());
    } catch (e) {
      error = e.message;
    }
  }

  const discoveryLabel = $derived(
    discovery < 25 ? 'Comfort zone' : discovery < 45 ? 'Mostly familiar' : discovery < 56 ? 'Balanced' : discovery < 80 ? 'Lean new' : 'Deep cuts & forgotten'
  );
</script>

<div class="layout">
  <Card title="Build a playlist" subtitle="Tune the mix, then export an .m3u">
    <div class="form">
      <div class="field-row">
        <label class="field">
          <span>Mood</span>
          <select bind:value={mood}>
            <option value="">Any mood</option>
            {#each appState.moods as m}<option value={m}>{titleCase(m)}</option>{/each}
          </select>
        </label>
        <label class="field">
          <span>Activity</span>
          <select bind:value={activity}>
            <option value="">Any activity</option>
            {#each appState.activities as a}<option value={a}>{titleCase(a)}</option>{/each}
          </select>
        </label>
      </div>

      <div class="field-row">
        <label class="field">
          <span>Era</span>
          <select bind:value={era}>
            {#each appState.eras as e}<option value={e}>{e === 'any' ? 'Any era' : titleCase(e)}</option>{/each}
          </select>
          {#if era !== 'any'}
            <label class="check small"><input type="checkbox" bind:checked={strictEra} /> Strict (exclude others)</label>
          {/if}
        </label>
        <div class="field">
          <span>Playlist size: {minSongs}–{maxSongs}</span>
          <div class="size-row">
            <input type="number" min="1" max="500" bind:value={minSongs} aria-label="min songs" />
            <span class="faint">to</span>
            <input type="number" min="1" max="500" bind:value={maxSongs} aria-label="max songs" />
          </div>
        </div>
      </div>

      <div class="field slider-field">
        <span class="slider-head">
          Energy
          <label class="check small"><input type="checkbox" bind:checked={useEnergy} /> set manually</label>
        </span>
        <input class="slider" type="range" min="0" max="100" bind:value={energy} disabled={!useEnergy} />
        <div class="scale faint"><span>Calm</span><span>{useEnergy ? energy + '%' : 'auto'}</span><span>Intense</span></div>
      </div>

      <div class="field slider-field">
        <span class="slider-head">Discovery <em class="muted">· {discoveryLabel}</em></span>
        <input class="slider" type="range" min="0" max="100" bind:value={discovery} />
        <div class="scale faint"><span>Familiar favourites</span><span>New & rare</span></div>
      </div>

      <div class="field">
        <span>Genres {#if selectedGenres.length}<em class="muted">· {selectedGenres.length} selected</em>{/if}</span>
        <div class="chips scroll">
          {#each appState.genres.slice(0, 40) as g}
            <button class="chip" class:on={selectedGenres.includes(g)} onclick={() => toggleGenre(g)}>{g}</button>
          {/each}
        </div>
      </div>

      <div class="field">
        <span>Artists</span>
        <div class="artist-input">
          <input type="text" placeholder="Search artists…" bind:value={artistQuery} />
          {#if artistMatches.length}
            <ul class="suggest">
              {#each artistMatches as a}
                <li><button onclick={() => addArtist(a)}>{a}</button></li>
              {/each}
            </ul>
          {/if}
        </div>
        {#if selectedArtists.length}
          <div class="chips">
            {#each selectedArtists as a}
              <button class="chip on" onclick={() => removeArtist(a)}>{a} ✕</button>
            {/each}
          </div>
        {/if}
      </div>

      <label class="check"><input type="checkbox" bind:checked={favoritesOnly} /> Favourites only</label>

      <div class="actions">
        <button class="btn btn-primary" onclick={() => generate(true)} disabled={loading}>
          {loading ? 'Generating…' : '✨ Generate playlist'}
        </button>
        {#if result}
          <button class="btn" onclick={() => generate(true)} disabled={loading}>↻ Reshuffle</button>
        {/if}
      </div>
      {#if error}<div class="error">{error}</div>{/if}
    </div>
  </Card>

  <Card title="Result" subtitle={result ? `${result.songs.length} tracks · ${result.candidates} candidates considered` : 'Your generated mix will appear here'}>
    {#if result}
      {#if result.notes && result.notes.length}
        {#each result.notes as n}<div class="note">{n}</div>{/each}
      {/if}
      {#if result.songs.length}
        <div class="export">
          <input class="name-input" type="text" placeholder={defaultName()} bind:value={playlistName} aria-label="playlist name" />
          <button class="btn btn-primary" onclick={download}>⬇ Export .m3u</button>
        </div>
        <ol class="tracks">
          {#each result.songs as s, i}
            <li>
              <span class="n">{i + 1}</span>
              <div class="t">
                <div class="t-title">{s.title || '(untitled)'}</div>
                <div class="t-sub muted">{s.artist || 'Unknown artist'}{s.year ? ' · ' + s.year : ''}{s.genre ? ' · ' + s.genre : ''}</div>
              </div>
              <div class="t-tags">
                {#if s.favorite}<span class="tag fav">♥</span>{/if}
                <span class="tag" title="energy">E {pct(s.energy)}</span>
                <span class="tag" title="valence">V {pct(s.valence)}</span>
                {#if s.matched}<span class="tag plays" title="play count">▶ {s.playCount}</span>{/if}
              </div>
            </li>
          {/each}
        </ol>
      {:else}
        <div class="empty">No tracks matched. Loosen your filters and try again.</div>
      {/if}
    {:else}
      <div class="empty">Set your mood, activity and filters, then hit generate.</div>
    {/if}
  </Card>
</div>

<style>
  .layout { display: grid; grid-template-columns: 400px 1fr; gap: 16px; align-items: start; }
  .form { display: flex; flex-direction: column; gap: 16px; }
  .field { display: flex; flex-direction: column; gap: 7px; }
  .field > span { font-size: 12.5px; font-weight: 600; color: var(--muted); text-transform: uppercase; letter-spacing: 0.05em; }
  .field-row { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }

  select, input[type='number'], input[type='text'] {
    background: var(--surface-3); color: var(--text);
    border: 1px solid var(--border-strong); border-radius: 10px;
    padding: 9px 11px; font: inherit; width: 100%;
  }
  select:focus, input:focus { outline: 2px solid var(--accent); outline-offset: -1px; }

  .size-row { display: flex; align-items: center; gap: 10px; }
  .size-row input { width: 90px; }

  .slider-head { display: flex; justify-content: space-between; align-items: center; }
  .slider { width: 100%; accent-color: var(--accent); }
  .scale { display: flex; justify-content: space-between; font-size: 11.5px; margin-top: 2px; }
  .scale span:nth-child(2) { color: var(--text); font-weight: 600; }

  .check { display: inline-flex; align-items: center; gap: 8px; font-size: 13.5px; color: var(--muted); cursor: pointer; text-transform: none; letter-spacing: 0; }
  .check.small { font-size: 12px; font-weight: 500; }
  .check input { accent-color: var(--accent); }

  .chips { display: flex; flex-wrap: wrap; gap: 7px; }
  .chips.scroll { max-height: 132px; overflow-y: auto; padding: 2px; }
  .chip {
    padding: 5px 11px; border-radius: 999px; font-size: 12.5px; font-weight: 550;
    background: var(--surface-3); border: 1px solid var(--border); color: var(--muted);
    transition: all 0.12s ease;
  }
  .chip:hover { border-color: var(--border-strong); color: var(--text); }
  .chip.on { background: color-mix(in srgb, var(--accent) 22%, transparent); border-color: var(--accent); color: #cfc7ff; }

  .artist-input { position: relative; }
  .suggest { position: absolute; z-index: 10; top: 100%; left: 0; right: 0; margin: 4px 0 0; padding: 4px; list-style: none;
    background: var(--surface-3); border: 1px solid var(--border-strong); border-radius: 10px; box-shadow: var(--shadow); }
  .suggest button { display: block; width: 100%; text-align: left; padding: 7px 9px; border-radius: 7px; }
  .suggest button:hover { background: var(--surface); }

  .actions { display: flex; gap: 10px; margin-top: 2px; }
  .error { color: var(--danger); font-size: 13px; }

  .export { display: flex; gap: 10px; margin-bottom: 14px; }
  .name-input { flex: 1; }
  .note { background: color-mix(in srgb, var(--accent-3) 14%, transparent); border: 1px solid color-mix(in srgb, var(--accent-3) 35%, transparent); color: #ffd9a0; padding: 8px 12px; border-radius: 10px; font-size: 13px; margin-bottom: 12px; }

  .tracks { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; }
  .tracks li { display: flex; align-items: center; gap: 12px; padding: 9px 6px; border-bottom: 1px solid var(--border); }
  .tracks li:last-child { border-bottom: none; }
  .n { width: 22px; text-align: right; color: var(--faint); font-variant-numeric: tabular-nums; flex: none; }
  .t { flex: 1; min-width: 0; }
  .t-title { font-weight: 550; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .t-sub { font-size: 12.5px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .t-tags { display: flex; gap: 6px; flex: none; }
  .tag { font-size: 11px; font-variant-numeric: tabular-nums; padding: 2px 7px; border-radius: 6px; background: var(--surface-3); color: var(--muted); }
  .tag.fav { color: var(--accent-4); }
  .tag.plays { color: var(--accent-2); }

  .empty { display: grid; place-items: center; min-height: 200px; color: var(--faint); text-align: center; padding: 0 20px; }

  @media (max-width: 900px) {
    .layout { grid-template-columns: 1fr; }
  }
</style>
