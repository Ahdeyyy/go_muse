<script>
  import { AreaChart, BarChart, DonutChart } from '@faintshadow/flarecharts';
  import Card from './Card.svelte';
  import Stat from './Stat.svelte';
  import BarList from './BarList.svelte';
  import { compact, num, hours, pct, titleCase } from '../lib/format.js';

  let { stats, hasBackup } = $props();

  const palette = ['#8b7cff', '#36d6c3', '#ffb454', '#ff6b9d', '#5aa9ff', '#b6f36c', '#c792ea', '#ff9f7a'];

  const featureOrder = ['energy', 'valence', 'danceability', 'acousticness', 'instrumentalness'];
  const featureData = $derived(
    featureOrder
      .filter((k) => stats.featureAvg && stats.featureAvg[k] != null)
      .map((k) => ({ label: titleCase(k).slice(0, 5), value: Math.round(stats.featureAvg[k] * 100) }))
  );

  const monthData = $derived((stats.playsByMonth || []).map((d) => ({ date: new Date(d.date), value: d.value })));
  const topArtists = $derived((stats.topArtists || []).map((d) => ({ label: d.label, value: d.value })));
  const topSongs = $derived(
    (stats.topSongs || []).map((d) => ({ label: d.title || '(untitled)', sub: d.artist, value: d.plays }))
  );
</script>

<div class="kpis">
  <Stat label="Tracks analyzed" value={num(stats.totalTracks)} hint="{num(stats.matchedTracks)} matched to listening data" />
  <Stat label="Total plays" value={compact(stats.totalPlays)} accent="var(--accent-2)" hint="across your history" />
  <Stat label="Listening time" value={hours(stats.listeningHours)} accent="var(--accent-3)" hint="from playback history" />
  <Stat label="Favorites" value={num(stats.favorites)} accent="var(--accent-4)" hint="{num(stats.distinctArtists)} artists · {num(stats.distinctGenres)} genres" />
</div>

{#if !hasBackup}
  <div class="hintbar card">
    <strong>Showing analysis only.</strong>
    <span class="muted">Upload a <code>.pxpl</code> backup to unlock listening-over-time, familiarity and most-played charts.</span>
  </div>
{/if}

<div class="grid">
  <div class="col-8">
    <Card title="Listening over time" subtitle="Plays per month from your history">
      <div class="chart h-tall">
        {#if monthData.length > 1}
          <AreaChart
            xAxis={{ type: 'time' }}
            plotOptions={{ curve: 'monotone', fillOpacity: 0.22 }}
            tooltip={true}
            series={[{ name: 'Plays', data: monthData, x: (d) => d.date, y: (d) => d.value }]}
          />
        {:else}
          <div class="empty">Upload a backup to see listening trends.</div>
        {/if}
      </div>
    </Card>
  </div>

  <div class="col-4">
    <Card title="Top genres" subtitle="Share of your library">
      <div class="chart h-tall">
        {#if (stats.genres || []).length}
          <DonutChart
            data={stats.genres}
            value={(d) => d.value}
            sliceLabel={(d) => d.label}
            colors={palette}
            centerCaption="tracks"
            labels="none"
          />
        {:else}
          <div class="empty">No genre data.</div>
        {/if}
      </div>
    </Card>
  </div>

  <div class="col-6">
    <Card title="Audio fingerprint" subtitle="Library-wide perceptual averages (%)">
      <div class="chart h-mid">
        {#if featureData.length}
          <BarChart
            yAxis={{ min: 0, max: 100 }}
            plotOptions={{ rx: 5, barPadding: 0.18 }}
            tooltip={true}
            series={[{ name: 'Average', data: featureData, x: (d) => d.label, y: (d) => d.value, color: 'var(--accent)' }]}
          />
        {:else}
          <div class="empty">No feature data.</div>
        {/if}
      </div>
    </Card>
  </div>

  <div class="col-6">
    <Card title="Familiarity spread" subtitle="How many tracks by play count">
      <div class="chart h-mid">
        {#if hasBackup && (stats.playCountHist || []).length}
          <BarChart
            plotOptions={{ rx: 5, barPadding: 0.16 }}
            tooltip={true}
            series={[{ name: 'Tracks', data: stats.playCountHist, x: (d) => d.label, y: (d) => d.value, color: 'var(--accent-2)' }]}
          />
        {:else}
          <div class="empty">Upload a backup to see play-count distribution.</div>
        {/if}
      </div>
    </Card>
  </div>

  <div class="col-6">
    <Card title="When you listen" subtitle="Plays by hour of day (UTC)">
      <div class="chart h-mid">
        {#if hasBackup && (stats.playsByHour || []).length}
          <BarChart
            xAxis={{ rotate: 0 }}
            plotOptions={{ rx: 4, barPadding: 0.12 }}
            tooltip={true}
            series={[{ name: 'Plays', data: stats.playsByHour, x: (d) => d.label, y: (d) => d.value, color: 'var(--accent-3)' }]}
          />
        {:else}
          <div class="empty">Upload a backup to see your daily rhythm.</div>
        {/if}
      </div>
    </Card>
  </div>

  <div class="col-6">
    <Card title="Energy distribution" subtitle="Track count by energy level">
      <div class="chart h-mid">
        {#if (stats.energyDist || []).length}
          <BarChart
            xAxis={{ rotate: -35 }}
            plotOptions={{ rx: 4, barPadding: 0.1 }}
            tooltip={true}
            series={[{ name: 'Tracks', data: stats.energyDist, x: (d) => d.label, y: (d) => d.value, color: 'var(--accent-4)' }]}
          />
        {:else}
          <div class="empty">No energy data.</div>
        {/if}
      </div>
    </Card>
  </div>

  <div class="col-6">
    <Card title="Top artists" subtitle="By total plays">
      {#if topArtists.length}
        <BarList items={topArtists} accent="var(--accent)" />
      {:else}
        <div class="empty pad">Upload a backup to rank your most-played artists.</div>
      {/if}
    </Card>
  </div>

  <div class="col-6">
    <Card title="Most played" subtitle="Your heaviest rotation">
      {#if topSongs.length}
        <BarList items={topSongs} accent="var(--accent-2)" />
      {:else}
        <div class="empty pad">Upload a backup to see your most-played tracks.</div>
      {/if}
    </Card>
  </div>
</div>

<style>
  .kpis { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; margin-bottom: 18px; }
  .hintbar { padding: 12px 16px; margin-bottom: 18px; display: flex; gap: 8px; flex-wrap: wrap; align-items: baseline; }
  .grid { display: grid; grid-template-columns: repeat(12, 1fr); gap: 16px; }
  .col-8 { grid-column: span 8; }
  .col-6 { grid-column: span 6; }
  .col-4 { grid-column: span 4; }
  .chart { width: 100%; }
  .h-tall { height: 300px; }
  .h-mid { height: 240px; }
  .empty { height: 100%; display: grid; place-items: center; color: var(--faint); text-align: center; padding: 0 16px; }
  .empty.pad { min-height: 120px; }
  code { background: var(--surface-3); padding: 1px 6px; border-radius: 5px; font-size: 12.5px; }

  @media (max-width: 960px) {
    .kpis { grid-template-columns: repeat(2, 1fr); }
    .col-8, .col-6, .col-4 { grid-column: span 12; }
  }
</style>
