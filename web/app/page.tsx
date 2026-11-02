'use client';

import React, { useState, useMemo, useEffect } from 'react';
import useSWR from 'swr';
import Image from 'next/image';
import { 
  Server as ServerIcon, 
  Users, 
  Search, 
  Copy, 
  Check, 
  Play, 
  Wifi, 
  Gamepad2,
  Database,
  Volume2,
  VolumeX,
  Star,
  RefreshCw,
  Zap,
  Globe,
  Radio,
  ExternalLink,
  ChevronRight,
  ChevronDown,
  ChevronUp,
  Folder,
  FolderOpen,
  X,
  Sliders,
  History,
  Info,
  Download,
  Terminal,
  ArrowUpDown,
  ArrowUp,
  ArrowDown,
  Sun,
  Moon
} from 'lucide-react';
import initialSnapshot from '@/lib/initialSnapshot.json';
import { ClassicThemeView } from '@/components/ClassicThemeView';
import { Button } from '@/components/ui/button';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { sfx } from '@/lib/audio';
import availableIcons from '@/public/icons/index.json';

const ICON_SET = new Set(availableIcons);

interface Player {
  name: string;
  frags: number;
  ping: number;
}

interface ServerData {
  id: string;
  ip: string;
  queryPort: number;
  gamePort: number;
  gamename: string;
  hostname: string;
  map: string;
  gametype: string;
  players: number;
  maxPlayers: number;
  ping: number;
  verified: boolean;
  lastChecked: string;
  roster?: Player[];
  cvars?: Record<string, string>;
}

interface SnapshotResponse {
  generatedAt: string;
  totalServers: number;
  totalPlayers: number;
  activeGames: number;
  servers: ServerData[];
}

interface CatalogGame {
  gamename: string;
  label: string;
  cipher?: string;
  port?: number;
  protocol?: string;
  category?: string;
}

type SortColumn = 'ping' | 'players' | 'game' | 'name' | 'map' | 'type' | 'ip';

const fetcher = (url: string) => fetch(url).then((res) => res.json());

// Strips Quake/Kingpin ^1..^9 color codes safely
function sanitizeText(str: string | undefined): string {
  if (!str) return '';
  return str.replace(/\^[0-9a-zA-Z]/g, '').trim();
}

const GAME_NAMES: Record<string, string> = {
  kingpin: 'Kingpin: Life of Crime',
  quake2: 'Quake II',
  quake3: 'Quake III Arena',
  q3a: 'Quake III Arena',
  ut: 'Unreal Tournament',
  ut99: 'Unreal Tournament 99',
  ut2004: 'Unreal Tournament 2004',
  deusex: 'Deus Ex',
  serioussam: 'Serious Sam',
  rune: 'Rune',
  mohaa: 'Medal of Honor',
  rtcw: 'Return to Castle Wolfenstein',
  et: 'Wolfenstein: Enemy Territory',
  wolfet: 'Wolfenstein: Enemy Territory',
  sin: 'SiN',
  alienarena: 'Alien Arena',
  bf1942: 'Battlefield 1942',
  bfield1942: 'Battlefield 1942',
  bfvietnam: 'Battlefield: Vietnam',
  bf2: 'Battlefield 2',
  cod: 'Call of Duty',
  cod2: 'Call of Duty 2',
  cod4: 'Call of Duty 4: Modern Warfare',
  coduo: 'Call of Duty: United Offensive',
  jk2: 'Jedi Outcast',
  jka: 'Jedi Academy',
  eliteforce: 'Star Trek: Elite Force',
  openarena: 'OpenArena',
  nexuiz: 'Nexuiz',
  warsow: 'Warsow',
  xonotic: 'Xonotic',
  halo: 'Halo CE',
  crysis: 'Crysis',
  quakeworld: 'QuakeWorld',
  swat4: 'SWAT 4'
};

function getGameDisplayName(gameKey: string): string {
  const g = gameKey?.toLowerCase() || '';
  return GAME_NAMES[g] || gameKey.toUpperCase();
}

function getGameIcon(gamename: string) {
  const g = gamename?.toLowerCase() || '';
  if (ICON_SET.has(g)) {
    return `/icons/${g}.png`;
  }
  if (g === 'ut') return '/icons/ut99.png';
  if (g === 'quake3arena' || g === 'q3a') return '/icons/quake3.png';
  if (g === 'q2') return '/icons/quake2.png';
  if (g === 'bfield1942') return '/icons/bf1942.png';
  if (g === 'wolfet') return '/icons/et.png';
  return null;
}

// Lightweight IP-to-Country Flag mapper
function getCountryFlag(ip: string): { flag: string; country: string } {
  if (!ip) return { flag: '🌐', country: 'Global' };
  const first = parseInt(ip.split('.')[0], 10);

  if ([15, 23, 24, 34, 50, 52, 54, 64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 96, 97, 98, 99, 104, 108, 142, 144, 158, 162, 172, 173, 174, 198, 199, 204, 205, 206, 207, 208, 209].includes(first)) {
    return { flag: '🇺🇸', country: 'North America' };
  }
  if ([37, 77, 85, 88, 91, 178, 185].includes(first)) {
    return { flag: '🇩🇪', country: 'Germany' };
  }
  if ([46, 51, 80, 82, 86].includes(first)) {
    return { flag: '🇬🇧', country: 'United Kingdom' };
  }
  if ([45, 94, 95, 193, 194, 212, 213, 217].includes(first)) {
    return { flag: '🇪🇺', country: 'Europe' };
  }
  if ([1, 14, 27, 36, 42, 43, 58, 59, 60, 61, 101, 103, 106, 110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120, 121, 122, 123, 124, 125, 126, 133, 175, 180, 182, 183, 202, 203, 210, 211, 218, 219, 220, 221, 222, 223].includes(first)) {
    return { flag: '🌏', country: 'Asia-Pacific' };
  }
  return { flag: '🌐', country: 'Global' };
}

export default function AllSeeingEyeDashboard() {
  const { data, isLoading, mutate } = useSWR<SnapshotResponse>(
    '/api/servers',
    fetcher,
    { 
      fallbackData: initialSnapshot as unknown as SnapshotResponse,
      refreshInterval: 10000, 
      revalidateOnFocus: true 
    }
  );

  // Theme: Default to original Modern Dark Theme ('dark'), with Authentic Classic 2002 Win32 as secondary ('classic')
  const [theme, setTheme] = useState<'dark' | 'classic'>('dark');

  useEffect(() => {
    try {
      const saved = localStorage.getItem('ase_theme');
      if (saved === 'classic' || saved === 'dark') {
        setTheme(saved);
      }
    } catch {}
  }, []);

  const handleSetTheme = (newTheme: 'dark' | 'classic') => {
    setTheme(newTheme);
    try {
      localStorage.setItem('ase_theme', newTheme);
    } catch {}
  };
  const [hideFull, setHideFull] = useState<boolean>(false);
  const [serverTypeFilter, setServerTypeFilter] = useState<string>('all');
  const [showServerTypeFolder, setShowServerTypeFolder] = useState<boolean>(true);
  const [leftSubTab, setLeftSubTab] = useState<'buddy' | 'server'>('server');
  const [rightSubTab, setRightSubTab] = useState<'players' | 'rules'>('players');

  // Sound FX State
  const [audioEnabled, setAudioEnabled] = useState<boolean>(false);

  // Selection & Inspector State
  const [selectedServerId, setSelectedServerId] = useState<string | null>(null);
  const [favorites, setFavorites] = useState<Set<string>>(new Set());
  const [history, setHistory] = useState<string[]>([]);
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [copiedRawId, setCopiedRawId] = useState<string | null>(null);

  // Tree Filters & Collapsible State
  const [treeSelection, setTreeSelection] = useState<string>('all');
  const [regionFilter, setRegionFilter] = useState<string>('all');
  const [searchQuery, setSearchQuery] = useState<string>('');
  const [hideEmpty, setHideEmpty] = useState<boolean>(false);

  // Sorting
  const [sortColumn, setSortColumn] = useState<SortColumn>('players');
  const [sortDirection, setSortDirection] = useState<'asc' | 'desc'>('desc');

  // Tree category toggles
  const [showGamesFolder, setShowGamesFolder] = useState<boolean>(true);
  const [showRegionsFolder, setShowRegionsFolder] = useState<boolean>(true);

  // Catalog Drawer
  const [catalogOpen, setCatalogOpen] = useState<boolean>(false);
  const [catalogSearch, setCatalogSearch] = useState<string>('');
  const [catalogCategory, setCatalogCategory] = useState<string>('all');

  // Server CVars Collapsible Dropdown
  const [cvarsOpen, setCvarsOpen] = useState<boolean>(false);

  // Load favorites & history from localStorage
  useEffect(() => {
    try {
      const savedFavs = localStorage.getItem('ase_favorites');
      if (savedFavs) {
        setFavorites(new Set(JSON.parse(savedFavs)));
      }
      const savedHistory = localStorage.getItem('ase_history');
      if (savedHistory) {
        setHistory(JSON.parse(savedHistory));
      }
    } catch {
      // LocalStorage fallback
    }
  }, []);

  const toggleFavorite = (id: string, e?: React.MouseEvent) => {
    e?.stopPropagation();
    sfx.playClick(900);
    setFavorites((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      try {
        localStorage.setItem('ase_favorites', JSON.stringify(Array.from(next)));
      } catch {}
      return next;
    });
  };

  const toggleAudio = () => {
    const next = !audioEnabled;
    setAudioEnabled(next);
    sfx.enabled = next;
    if (next) sfx.playSuccess();
  };

  // Active game distribution
  const activeGameDistribution = useMemo(() => {
    if (!data?.servers) return [];
    const counts: Record<string, { count: number; players: number }> = {};
    for (const srv of data.servers) {
      const g = srv.gamename?.toLowerCase() || 'unknown';
      if (!counts[g]) counts[g] = { count: 0, players: 0 };
      counts[g].count++;
      counts[g].players += srv.players || 0;
    }
    return Object.entries(counts)
      .map(([gameKey, stat]) => ({
        gameKey,
        displayName: getGameDisplayName(gameKey),
        serverCount: stat.count,
        playerCount: stat.players,
        icon: getGameIcon(gameKey),
      }))
      .sort((a, b) => b.serverCount - a.serverCount);
  }, [data?.servers]);

  // Handle Header Click Sorting
  const handleSort = (col: SortColumn) => {
    sfx.playClick(1100);
    if (sortColumn === col) {
      setSortDirection((prev) => (prev === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortColumn(col);
      // Sensible defaults: ping ascending (lowest first), players descending (most first), others ascending
      setSortDirection(col === 'ping' ? 'asc' : col === 'players' ? 'desc' : 'asc');
    }
  };

  // Filtered & Sorted servers
  const filteredServers = useMemo(() => {
    if (!data?.servers) return [];

    const rawFiltered = data.servers.filter((srv) => {
      // Tree folder selection
      if (treeSelection === 'favorites') {
        if (!favorites.has(srv.id)) return false;
      } else if (treeSelection === 'history') {
        if (!history.includes(srv.id)) return false;
      } else if (treeSelection !== 'all') {
        if (srv.gamename?.toLowerCase() !== treeSelection.toLowerCase()) return false;
      }

      // Region / Territory filter
      if (regionFilter !== 'all') {
        const { country } = getCountryFlag(srv.ip);
        if (country !== regionFilter) return false;
      }

      // Hide empty
      if (hideEmpty && srv.players === 0) return false;

      // Search text
      if (searchQuery.trim() !== '') {
        const q = searchQuery.toLowerCase();
        const nameMatch = sanitizeText(srv.hostname).toLowerCase().includes(q);
        const mapMatch = sanitizeText(srv.map).toLowerCase().includes(q);
        const ipMatch = srv.ip.toLowerCase().includes(q);
        const gameMatch = srv.gamename.toLowerCase().includes(q);
        if (!nameMatch && !mapMatch && !ipMatch && !gameMatch) return false;
      }

      return true;
    });

    // Deduplicate servers by game + IP + effective gameport / clean hostname
    const deduped: ServerData[] = [];
    const seen = new Set<string>();

    for (const srv of rawFiltered) {
      const gName = (srv.gamename || '').toLowerCase();
      let effectivePort = srv.gamePort || srv.queryPort;
      if (gName === 'kingpin' && srv.gamePort >= 31520 && srv.queryPort >= 31510 && srv.queryPort <= 31599) {
        effectivePort = srv.queryPort;
      }
      const portKey = `${gName}:${srv.ip}:${effectivePort}`;
      const cleanHost = sanitizeText(srv.hostname);
      const nameKey = cleanHost ? `${gName}:${srv.ip}:${cleanHost}` : null;

      if (seen.has(portKey) || (nameKey && seen.has(nameKey))) {
        continue;
      }
      seen.add(portKey);
      if (nameKey) seen.add(nameKey);
      deduped.push(srv);
    }

    return deduped.sort((a, b) => {
        let diff = 0;
        if (sortColumn === 'players') {
          diff = a.players - b.players;
        } else if (sortColumn === 'ping') {
          diff = a.ping - b.ping;
        } else if (sortColumn === 'game') {
          diff = a.gamename.localeCompare(b.gamename);
        } else if (sortColumn === 'name') {
          diff = sanitizeText(a.hostname).localeCompare(sanitizeText(b.hostname));
        } else if (sortColumn === 'map') {
          diff = sanitizeText(a.map).localeCompare(sanitizeText(b.map));
        } else if (sortColumn === 'type') {
          diff = (a.gametype || '').localeCompare(b.gametype || '');
        } else if (sortColumn === 'ip') {
          diff = a.ip.localeCompare(b.ip);
        }
        return sortDirection === 'asc' ? diff : -diff;
      });
  }, [data?.servers, treeSelection, regionFilter, hideEmpty, searchQuery, sortColumn, sortDirection, favorites, history]);

  // Ensure an active selected server
  const selectedServer = useMemo(() => {
    if (!filteredServers || filteredServers.length === 0) return null;
    if (selectedServerId) {
      const found = filteredServers.find((s) => s.id === selectedServerId);
      if (found) return found;
    }
    return filteredServers[0];
  }, [filteredServers, selectedServerId]);

  const handleSelectServer = (srv: ServerData) => {
    sfx.playClick(1000);
    setSelectedServerId(srv.id);

    // Track in history
    setHistory((prev) => {
      const next = [srv.id, ...prev.filter((id) => id !== srv.id)].slice(0, 30);
      try {
        localStorage.setItem('ase_history', JSON.stringify(next));
      } catch {}
      return next;
    });
  };

  const handleConnect = (srv: ServerData) => {
    sfx.playSuccess();
    const addr = `${srv.ip}:${srv.gamePort || srv.queryPort}`;
    navigator.clipboard.writeText(`/connect ${addr}`);
    setCopiedId(srv.id);
    setTimeout(() => {
      setCopiedId((prev) => (prev === srv.id ? null : prev));
    }, 2500);
  };

  const handleCopyRawIP = (srv: ServerData) => {
    sfx.playSuccess();
    const addr = `${srv.ip}:${srv.gamePort || srv.queryPort}`;
    navigator.clipboard.writeText(addr);
    setCopiedRawId(srv.id);
    setTimeout(() => {
      setCopiedRawId((prev) => (prev === srv.id ? null : prev));
    }, 2500);
  };

  // Export current list to text file
  const handleExportList = () => {
    sfx.playSuccess();
    const lines = filteredServers.map((s) => `${s.ip}:${s.gamePort || s.queryPort} # ${sanitizeText(s.hostname)} [${s.gamename}]`).join('\n');
    const header = `# The All-Seeing Eye Server List Export\n# Total: ${filteredServers.length} servers\n# Exported at: ${new Date().toISOString()}\n\n`;
    const blob = new Blob([header + lines], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `ase-servers-${treeSelection !== 'all' ? treeSelection : 'all'}.txt`;
    link.click();
    URL.revokeObjectURL(url);
  };

  // Global Keyboard Navigation (Arrows Up/Down, Enter, Escape, /)
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (['INPUT', 'TEXTAREA'].includes((e.target as HTMLElement)?.tagName)) {
        if (e.key === 'Escape') {
          (e.target as HTMLElement).blur();
        }
        return;
      }

      if (e.key === '/') {
        e.preventDefault();
        document.getElementById('ase-filter-search')?.focus();
        return;
      }

      if (e.key === 'Escape') {
        setSearchQuery('');
        setCatalogOpen(false);
        return;
      }

      if (!filteredServers || filteredServers.length === 0) return;

      const currentIndex = filteredServers.findIndex((s) => s.id === selectedServer?.id);

      if (e.key === 'ArrowDown' || e.key === 'j') {
        e.preventDefault();
        const nextIndex = currentIndex < filteredServers.length - 1 ? currentIndex + 1 : 0;
        const nextServer = filteredServers[nextIndex];
        setSelectedServerId(nextServer.id);
        sfx.playClick(1000);
      } else if (e.key === 'ArrowUp' || e.key === 'k') {
        e.preventDefault();
        const prevIndex = currentIndex > 0 ? currentIndex - 1 : filteredServers.length - 1;
        const prevServer = filteredServers[prevIndex];
        setSelectedServerId(prevServer.id);
        sfx.playClick(1000);
      } else if (e.key === 'Enter') {
        if (selectedServer) {
          e.preventDefault();
          handleConnect(selectedServer);
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [filteredServers, selectedServer]);

  // Fetch catalog from backend API
  const { data: catalogData } = useSWR<{ total: number; games: CatalogGame[] }>(
    `/api/games?q=${encodeURIComponent(catalogSearch)}&category=${encodeURIComponent(catalogCategory)}&limit=100`,
    fetcher
  );

  if (theme === 'classic') {
    return (
      <>
        <ClassicThemeView
          data={data}
          isLoading={isLoading}
          mutate={mutate}
          setTheme={handleSetTheme}
          audioEnabled={audioEnabled}
          toggleAudio={toggleAudio}
          favorites={favorites}
          toggleFavorite={toggleFavorite}
          history={history}
          selectedServer={selectedServer}
          handleSelectServer={handleSelectServer}
          handleConnect={handleConnect}
          handleCopyRawIP={handleCopyRawIP}
          treeSelection={treeSelection}
          setTreeSelection={setTreeSelection}
          regionFilter={regionFilter}
          setRegionFilter={setRegionFilter}
          searchQuery={searchQuery}
          setSearchQuery={setSearchQuery}
          hideEmpty={hideEmpty}
          setHideEmpty={setHideEmpty}
          hideFull={hideFull}
          setHideFull={setHideFull}
          serverTypeFilter={serverTypeFilter}
          setServerTypeFilter={setServerTypeFilter}
          sortColumn={sortColumn}
          sortDirection={sortDirection}
          handleSort={handleSort}
          showGamesFolder={showGamesFolder}
          setShowGamesFolder={setShowGamesFolder}
          showRegionsFolder={showRegionsFolder}
          setShowRegionsFolder={setShowRegionsFolder}
          showServerTypeFolder={showServerTypeFolder}
          setShowServerTypeFolder={setShowServerTypeFolder}
          leftSubTab={leftSubTab}
          setLeftSubTab={setLeftSubTab}
          rightSubTab={rightSubTab}
          setRightSubTab={setRightSubTab}
          filteredServers={filteredServers}
          activeGameDistribution={activeGameDistribution}
          getCountryFlag={getCountryFlag}
          sanitizeText={sanitizeText}
          copiedId={copiedId}
          copiedRawId={copiedRawId}
          setCatalogOpen={setCatalogOpen}
        />

        {/* Catalog Dossier Drawer (4,678 Games Directory) */}
        <Sheet open={catalogOpen} onOpenChange={setCatalogOpen}>
          <SheetContent 
            side="right" 
            className="w-full sm:max-w-xl bg-slate-950 border-l border-sky-500/30 text-slate-100 p-0 flex flex-col font-mono"
          >
            <SheetHeader className="p-4 border-b border-sky-500/20 bg-slate-900/60">
              <div className="flex items-center gap-2">
                <Database className="size-5 text-sky-400" />
                <SheetTitle className="font-mono text-base font-bold text-sky-300 uppercase tracking-wider">
                  All-Seeing Eye Game Catalog (4,678 Titles)
                </SheetTitle>
              </div>
              <p className="text-xs text-slate-400 mt-1">
                Search and filter across all 4,678 cataloged GameSpy, Quake, and Unreal multiplayer games.
              </p>
            </SheetHeader>

            <div className="p-4 border-b border-slate-800 space-y-3 bg-slate-950">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-sky-400/60" />
                <input
                  type="text"
                  value={catalogSearch}
                  onChange={(e) => setCatalogSearch(e.target.value)}
                  placeholder="SEARCH TITLES (E.G. DOOM, QUAKE, COD, BATTLEFIELD)..."
                  className="w-full pl-9 pr-4 py-2 rounded-lg bg-slate-900 border border-slate-800 focus:border-sky-400 focus:outline-none text-slate-200 text-xs font-mono"
                />
              </div>

              <div className="flex items-center gap-1.5 overflow-x-auto pb-1 text-[11px]">
                {['all', 'GameSpy PC Classic', 'idTech / Quake', 'EA / DICE', 'Nintendo DWC', 'Unreal Engine'].map((cat) => (
                  <button
                    key={cat}
                    onClick={() => {
                      sfx.playClick();
                      setCatalogCategory(cat);
                    }}
                    className={`px-2 py-1 rounded whitespace-nowrap border transition-all ${
                      catalogCategory === cat
                        ? 'bg-sky-500/20 border-sky-400 text-sky-300 font-bold'
                        : 'bg-slate-900 border-slate-800 text-slate-400 hover:text-slate-200'
                    }`}
                  >
                    {cat === 'all' ? 'All Sectors' : cat}
                  </button>
                ))}
              </div>
            </div>

            <div className="flex-1 overflow-y-auto p-4 space-y-2">
              {catalogData?.games?.map((g) => {
                const icon = getGameIcon(g.gamename);
                const isSelected = treeSelection.toLowerCase() === g.gamename.toLowerCase();

                return (
                  <div
                    key={g.gamename}
                    className={`p-2.5 rounded-lg border transition-all flex items-center justify-between gap-3 ${
                      isSelected
                        ? 'bg-sky-950/60 border-sky-400 text-sky-200'
                        : 'bg-slate-900/50 border-slate-800/80 hover:border-slate-700'
                    }`}
                  >
                    <div className="flex items-center gap-2.5 min-w-0">
                      <div className="relative size-9 rounded border border-slate-800 bg-slate-950 overflow-hidden shrink-0">
                        {icon ? (
                          <Image src={icon} alt={g.label} fill sizes="36px" className="object-cover" />
                        ) : (
                          <Gamepad2 className="size-5 m-2 text-slate-600" />
                        )}
                      </div>
                      <div className="min-w-0">
                        <div className="font-bold text-xs text-slate-100 truncate">{g.label}</div>
                        <div className="flex items-center gap-1.5 text-[10px] text-slate-500 mt-0.5">
                          <span>{g.gamename}</span>
                          <span>•</span>
                          <span className="text-sky-400/80">{g.category || 'Classic'}</span>
                        </div>
                      </div>
                    </div>

                    <button
                      onClick={() => {
                        sfx.playDeploy();
                        setTreeSelection(g.gamename);
                        setCatalogOpen(false);
                      }}
                      className="px-2.5 py-1 rounded text-[11px] font-mono bg-sky-950 border border-sky-500/30 text-sky-300 hover:bg-sky-900 hover:border-sky-400 transition-colors shrink-0"
                    >
                      SELECT
                    </button>
                  </div>
                );
              })}
            </div>
          </SheetContent>
        </Sheet>
      </>
    );
  }

  return (
    <div className="h-screen max-h-screen h-dvh max-h-dvh bg-[#0d1117] text-slate-100 font-sans select-none flex flex-col overflow-hidden">

      {/* =========================================================================
          ASE NATIVE APPLICATION TITLE & TELEMETRY BAR
         ========================================================================= */}
      <div className="shrink-0 ase-menubar px-3 py-1 flex items-center justify-between border-b border-slate-700/80">
        <div className="flex items-center gap-2.5">
          {/* The Iconic All-Seeing Eye Logo */}
          <div className="relative size-6 rounded overflow-hidden border border-sky-400/50 shadow-[0_0_10px_rgba(0,210,255,0.4)] shrink-0">
            <Image 
              src="/ase-logo.png" 
              alt="The All-Seeing Eye Logo" 
              fill 
              sizes="24px" 
              className="object-cover" 
            />
          </div>
          <span className="font-bold text-xs text-slate-100 tracking-wide">
            The All-Seeing Eye - [{data?.totalServers?.toLocaleString() || '0'} servers, {data?.totalPlayers?.toLocaleString() || '0'} players online]
          </span>
        </div>

        {/* Live Telemetry Status Chips */}
        <div className="hidden md:flex items-center gap-2 font-mono text-[11px]">
          <div className="flex items-center gap-1 px-2 py-0.5 rounded bg-slate-900/90 border border-slate-700/80 text-slate-300">
            <span className="text-slate-500">WORKERS:</span>
            <span className="text-sky-400 font-semibold">64 POOLED</span>
          </div>

          <div className="flex items-center gap-1 px-2 py-0.5 rounded bg-slate-900/90 border border-slate-700/80 text-slate-300">
            <span className="text-slate-500">CACHE:</span>
            <span className="text-emerald-400 font-semibold">retro-server-mirror</span>
          </div>

          <div className="hidden xl:flex items-center gap-1 px-2 py-0.5 rounded bg-slate-900/90 border border-slate-700/80 text-slate-300">
            <span className="text-slate-500">PROTOCOLS:</span>
            <span className="text-amber-400 font-semibold">IDTECH 2/3 &amp; GS</span>
          </div>

          <div className="hidden lg:flex items-center gap-1 px-2 py-0.5 rounded bg-slate-900/90 border border-slate-700/80 text-slate-300">
            <span className="text-slate-500">CADENCE:</span>
            <span className="text-sky-300 font-semibold">10.0s</span>
          </div>
        </div>

        {/* Status Diode & Audio Switch */}
        <div className="flex items-center gap-2">
          <button
            onClick={toggleAudio}
            className={`flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-mono border transition-all ${
              audioEnabled 
                ? 'bg-sky-500/20 border-sky-400 text-sky-300' 
                : 'bg-slate-900 border-slate-700 text-slate-400'
            }`}
            title="Toggle synthesized Web Audio FX"
          >
            {audioEnabled ? <Volume2 className="size-3" /> : <VolumeX className="size-3" />}
            <span>SFX</span>
          </button>

          <div className="flex items-center gap-1.5 px-2 py-0.5 rounded bg-slate-900 border border-emerald-500/40 text-[10px] font-mono text-emerald-400">
            <span className="size-1.5 rounded-full bg-emerald-400 animate-pulse"></span>
            <span>ASE Net: Active</span>
          </div>
        </div>
      </div>

      {/* =========================================================================
          THE ICONIC 2000s BUTTON TOOLBAR
         ========================================================================= */}
      <div className="shrink-0 ase-toolbar px-3 py-1 flex items-center flex-wrap gap-2 text-xs border-b border-slate-900">
        {/* Refresh Master Button */}
        <button 
          onClick={() => {
            sfx.playSweep();
            mutate();
          }}
          className="ase-toolbar-btn px-2.5 py-1 rounded text-slate-200 hover:text-white flex items-center gap-1.5 font-semibold transition-all"
          title="Refresh server lists from master"
        >
          <span className="text-sm">👁️</span>
          <span>Refresh Master</span>
        </button>

        {/* Ping Visible Button */}
        <button 
          onClick={() => {
            sfx.playClick(1200);
            mutate();
          }}
          className="ase-toolbar-btn px-2.5 py-1 rounded text-slate-200 hover:text-white flex items-center gap-1.5 font-semibold transition-all"
          title="Broadcast immediate UDP ping check"
        >
          <Zap className="size-3.5 text-yellow-400 fill-yellow-400" />
          <span>Ping Visible</span>
        </button>

        {/* Launch / Connect Button */}
        <button 
          onClick={() => selectedServer && handleConnect(selectedServer)}
          disabled={!selectedServer}
          className={`ase-toolbar-btn px-2.5 py-1 rounded font-bold transition-all flex items-center gap-1.5 ${
            selectedServer 
              ? 'text-emerald-300 border-emerald-500/40 hover:text-emerald-200' 
              : 'text-slate-500 border-slate-800 opacity-60 cursor-not-allowed'
          }`}
          title="Copy /connect command and trigger game client"
        >
          <Play className="size-3 fill-emerald-400 text-emerald-400" />
          <span>{copiedId === selectedServer?.id ? 'COPIED!' : 'Connect to Game'}</span>
        </button>

        {/* Add/Remove Favorite */}
        <button 
          onClick={(e) => selectedServer && toggleFavorite(selectedServer.id, e)}
          disabled={!selectedServer}
          className="ase-toolbar-btn px-2.5 py-1 rounded text-amber-300 hover:text-white flex items-center gap-1.5 transition-all"
        >
          <Star className={`size-3.5 ${selectedServer && favorites.has(selectedServer.id) ? 'fill-amber-400 text-amber-400' : ''}`} />
          <span>{selectedServer && favorites.has(selectedServer.id) ? 'Favorited' : 'Add Favorite'}</span>
        </button>

        {/* Export Server List */}
        <button
          onClick={handleExportList}
          className="ase-toolbar-btn px-2 py-1 rounded text-slate-300 hover:text-white flex items-center gap-1 transition-all"
          title="Export visible server list as .txt"
        >
          <Download className="size-3.5 text-sky-400" />
          <span>Export .txt</span>
        </button>

        {/* Theme Switcher Button */}
        <button
          onClick={() => {
            sfx.playClick();
            handleSetTheme('classic');
          }}
          className="ase-toolbar-btn px-2.5 py-1 rounded text-amber-300 hover:text-white flex items-center gap-1.5 font-semibold transition-all"
          title="Switch to Authentic 2002 Win32 Theme"
        >
          <Sun className="size-3.5 text-yellow-400" />
          <span>Classic 2002</span>
        </button>

        <div className="h-5 w-[1px] bg-slate-700/80 mx-1 hidden sm:block"></div>

        {/* Live Filter Box */}
        <div className="flex items-center gap-1.5 bg-[#12161f] border border-slate-700 px-2.5 py-0.5 rounded text-xs">
          <Search className="size-3.5 text-slate-400" />
          <input 
            id="ase-filter-search"
            type="text" 
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search server, map, IP (Press '/' to focus)..." 
            className="bg-transparent text-white text-[11px] font-mono focus:outline-none w-44 sm:w-56 placeholder:text-slate-500"
          />
          {searchQuery && (
            <button onClick={() => setSearchQuery('')} className="text-slate-400 hover:text-white">
              <X className="size-3" />
            </button>
          )}
        </div>

        {/* Populated Toggle */}
        <button
          onClick={() => {
            sfx.playClick();
            setHideEmpty(!hideEmpty);
          }}
          className={`px-2 py-1 rounded text-[11px] font-mono border transition-all ${
            hideEmpty
              ? 'bg-sky-500/20 border-sky-400 text-sky-300 font-bold'
              : 'bg-slate-900 border-slate-700 text-slate-400 hover:text-slate-200'
          }`}
        >
          [ Not Empty ]
        </button>

        {/* Keyboard hints badge */}
        <span className="hidden xl:inline text-[10px] text-slate-500 font-mono">
          Keys: ↑/↓ select, Enter connect
        </span>

        {/* Master Catalog Drawer Trigger */}
        <button
          onClick={() => {
            sfx.playDeploy();
            setCatalogOpen(true);
          }}
          className="ml-auto ase-toolbar-btn px-2.5 py-1 rounded text-sky-300 hover:text-white flex items-center gap-1.5 text-xs font-semibold"
        >
          <Database className="size-3.5" />
          <span>4,678 Games Catalog ▼</span>
        </button>
      </div>

      {/* =========================================================================
          ASE MAIN 3-PANE APPLICATION BODY
         ========================================================================= */}
      <div className="flex-1 min-h-0 grid grid-cols-1 md:grid-cols-12 overflow-hidden">

        {/* ---------------------------------------------------------------------
            LEFT PANE: THE CLASSIC ALL-SEEING EYE DIRECTORY TREE (3 COLS)
           --------------------------------------------------------------------- */}
        <div className="md:col-span-3 h-full ase-tree-panel p-2 flex flex-col min-h-0 text-xs select-none overflow-hidden">
          
          {/* Main Folders */}
          <div className="shrink-0 mb-3">
            <div className="font-bold text-[11px] text-sky-400 uppercase tracking-wider mb-1 flex items-center gap-1 font-mono">
              <FolderOpen className="size-3.5 text-sky-400" />
              <span>Scanner Categories</span>
            </div>
            <div className="space-y-0.5 pl-1.5 font-mono text-[11px]">
              {/* All Servers */}
              <div 
                onClick={() => {
                  sfx.playClick();
                  setTreeSelection('all');
                  setRegionFilter('all');
                }}
                className={`px-2 py-1 rounded cursor-pointer flex items-center justify-between transition-colors ${
                  treeSelection === 'all' && regionFilter === 'all'
                    ? 'ase-tree-item-active' 
                    : 'text-slate-300 hover:bg-slate-800/60'
                }`}
              >
                <div className="flex items-center gap-1.5 truncate">
                  <Folder className="size-3 text-sky-400" />
                  <span>All Active Servers</span>
                </div>
                <span className="text-[10px] bg-slate-800 px-1.5 py-0.2 rounded text-slate-300 font-bold">
                  {data?.totalServers || 0}
                </span>
              </div>

              {/* Favorites */}
              <div 
                onClick={() => {
                  sfx.playClick();
                  setTreeSelection('favorites');
                }}
                className={`px-2 py-1 rounded cursor-pointer flex items-center justify-between transition-colors ${
                  treeSelection === 'favorites' 
                    ? 'ase-tree-item-active' 
                    : 'text-slate-300 hover:bg-slate-800/60'
                }`}
              >
                <div className="flex items-center gap-1.5 truncate">
                  <Star className="size-3 text-amber-400 fill-amber-400" />
                  <span>Favorites List</span>
                </div>
                <span className="text-[10px] bg-slate-800 px-1.5 py-0.2 rounded text-slate-300 font-bold">
                  {favorites.size}
                </span>
              </div>

              {/* History */}
              <div 
                onClick={() => {
                  sfx.playClick();
                  setTreeSelection('history');
                }}
                className={`px-2 py-1 rounded cursor-pointer flex items-center justify-between transition-colors ${
                  treeSelection === 'history' 
                    ? 'ase-tree-item-active' 
                    : 'text-slate-300 hover:bg-slate-800/60'
                }`}
              >
                <div className="flex items-center gap-1.5 truncate">
                  <History className="size-3 text-slate-400" />
                  <span>Recent History</span>
                </div>
                <span className="text-[10px] bg-slate-800 px-1.5 py-0.2 rounded text-slate-300 font-bold">
                  {history.length}
                </span>
              </div>
            </div>
          </div>

          {/* Active Theaters (Per-Game Branch) - Scales dynamically to fill available vertical space */}
          <div className={`flex flex-col min-h-0 ${showGamesFolder ? 'flex-1 mb-2' : 'shrink-0 mb-2'}`}>
            <div 
              onClick={() => setShowGamesFolder(!showGamesFolder)}
              className="shrink-0 font-bold text-[11px] text-sky-400 uppercase tracking-wider mb-1 flex items-center justify-between cursor-pointer font-mono hover:text-sky-300"
            >
              <span className="flex items-center gap-1">
                <Gamepad2 className="size-3.5 text-sky-400" />
                <span>Active Games ({activeGameDistribution.length})</span>
              </span>
              <span>{showGamesFolder ? <ChevronDown className="size-3" /> : <ChevronRight className="size-3" />}</span>
            </div>

            {showGamesFolder && (
              <div className="flex-1 min-h-0 space-y-0.5 pl-1.5 font-mono text-[11px] overflow-y-auto pr-1 scrollbar-thin">
                {activeGameDistribution.map((game) => {
                  const isSelected = treeSelection.toLowerCase() === game.gameKey.toLowerCase();
                  return (
                    <div
                      key={game.gameKey}
                      onClick={() => {
                        sfx.playClick();
                        setTreeSelection(game.gameKey);
                        setRegionFilter('all');
                      }}
                      className={`px-2 py-1 rounded cursor-pointer flex items-center justify-between transition-colors ${
                        isSelected 
                          ? 'ase-tree-item-active' 
                          : 'text-slate-300 hover:bg-slate-800/60'
                      }`}
                    >
                      <div className="flex items-center gap-1.5 truncate">
                        {game.icon ? (
                          <div className="relative size-3.5 rounded overflow-hidden shrink-0">
                            <Image src={game.icon} alt={game.displayName} fill sizes="14px" className="object-cover" />
                          </div>
                        ) : (
                          <Gamepad2 className="size-3 text-slate-500 shrink-0" />
                        )}
                        <span className="truncate">{game.displayName}</span>
                      </div>
                      <span className="text-[10px] bg-slate-800 px-1.5 py-0.2 rounded text-slate-300 shrink-0 font-bold">
                        {game.serverCount}
                      </span>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          {/* By Country / Territory Branch */}
          <div className="shrink-0 pt-2 border-t border-slate-800/80">
            <div 
              onClick={() => setShowRegionsFolder(!showRegionsFolder)}
              className="shrink-0 font-bold text-[11px] text-sky-400 uppercase tracking-wider mb-1 flex items-center justify-between cursor-pointer font-mono hover:text-sky-300"
            >
              <span className="flex items-center gap-1">
                <Globe className="size-3.5 text-sky-400" />
                <span>By Territory</span>
              </span>
              <span>{showRegionsFolder ? <ChevronDown className="size-3" /> : <ChevronRight className="size-3" />}</span>
            </div>

            {showRegionsFolder && (
              <div className="space-y-0.5 pl-1.5 font-mono text-[11px] max-h-36 overflow-y-auto pr-1 scrollbar-thin">
                {[
                  { name: 'North America', flag: '🇺🇸' },
                  { name: 'Europe', flag: '🇪🇺' },
                  { name: 'Germany', flag: '🇩🇪' },
                  { name: 'United Kingdom', flag: '🇬🇧' },
                  { name: 'Asia-Pacific', flag: '🌏' }
                ].map((reg) => (
                  <div
                    key={reg.name}
                    onClick={() => {
                      sfx.playClick();
                      setRegionFilter(regionFilter === reg.name ? 'all' : reg.name);
                    }}
                    className={`px-2 py-1 rounded cursor-pointer flex items-center justify-between transition-colors ${
                      regionFilter === reg.name 
                        ? 'ase-tree-item-active' 
                        : 'text-slate-400 hover:bg-slate-800/60 hover:text-slate-200'
                    }`}
                  >
                    <div className="flex items-center gap-1.5 truncate">
                      <span>{reg.flag}</span>
                      <span>{reg.name}</span>
                    </div>
                    {regionFilter === reg.name && (
                      <span className="text-[10px] text-sky-400 font-bold">ACTIVE</span>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>

        </div>

        {/* ---------------------------------------------------------------------
            RIGHT PANE: ICONIC SERVER SPREADSHEET & DUAL INSPECTOR (9 COLS)
           --------------------------------------------------------------------- */}
        <div className="md:col-span-9 h-full flex flex-col min-h-0 ase-grid-panel overflow-hidden">
          
          {/* Top Half: Dense Server Spreadsheet Grid with Clickable Sort Headers */}
          <div className="flex-1 min-h-[140px] overflow-auto border-b-2 border-slate-700/80 scrollbar-thin">
            <table className="w-full text-left text-xs border-collapse">
              <thead className="bg-[#181f2a] text-slate-300 text-[11px] uppercase font-bold sticky top-0 border-b border-slate-700 select-none shadow-sm z-10">
                <tr>
                  <th className="py-1.5 px-2 border-r border-slate-700/60 w-7 text-center">⭐</th>
                  <th className="py-1.5 px-2 border-r border-slate-700/60 w-8 text-center">Loc</th>
                  
                  {/* Clickable Game Column */}
                  <th 
                    onClick={() => handleSort('game')} 
                    className="py-1.5 px-2 border-r border-slate-700/60 w-24 cursor-pointer hover:bg-slate-700/50 hover:text-white transition-colors"
                  >
                    <div className="flex items-center gap-1">
                      <span>Game</span>
                      {sortColumn === 'game' && <span className="text-sky-400 font-bold">{sortDirection === 'asc' ? '▲' : '▼'}</span>}
                    </div>
                  </th>

                  {/* Clickable Name Column */}
                  <th 
                    onClick={() => handleSort('name')} 
                    className="py-1.5 px-2 border-r border-slate-700/60 cursor-pointer hover:bg-slate-700/50 hover:text-white transition-colors"
                  >
                    <div className="flex items-center gap-1">
                      <span>Server Name</span>
                      {sortColumn === 'name' && <span className="text-sky-400 font-bold">{sortDirection === 'asc' ? '▲' : '▼'}</span>}
                    </div>
                  </th>

                  {/* Clickable Ping Column */}
                  <th 
                    onClick={() => handleSort('ping')} 
                    className="py-1.5 px-2 border-r border-slate-700/60 text-right w-16 cursor-pointer hover:bg-slate-700/50 hover:text-white transition-colors"
                  >
                    <div className="flex items-center justify-end gap-1">
                      <span>Ping</span>
                      {sortColumn === 'ping' && <span className="text-sky-400 font-bold">{sortDirection === 'asc' ? '▲' : '▼'}</span>}
                    </div>
                  </th>

                  {/* Clickable Players Column */}
                  <th 
                    onClick={() => handleSort('players')} 
                    className="py-1.5 px-2 border-r border-slate-700/60 text-center w-20 cursor-pointer hover:bg-slate-700/50 hover:text-white transition-colors"
                  >
                    <div className="flex items-center justify-center gap-1">
                      <span>Players</span>
                      {sortColumn === 'players' && <span className="text-sky-400 font-bold">{sortDirection === 'asc' ? '▲' : '▼'}</span>}
                    </div>
                  </th>

                  {/* Clickable Map Column */}
                  <th 
                    onClick={() => handleSort('map')} 
                    className="py-1.5 px-2 border-r border-slate-700/60 w-32 cursor-pointer hover:bg-slate-700/50 hover:text-white transition-colors"
                  >
                    <div className="flex items-center gap-1">
                      <span>Map</span>
                      {sortColumn === 'map' && <span className="text-sky-400 font-bold">{sortDirection === 'asc' ? '▲' : '▼'}</span>}
                    </div>
                  </th>

                  {/* Clickable Gametype Column */}
                  <th 
                    onClick={() => handleSort('type')} 
                    className="py-1.5 px-2 border-r border-slate-700/60 w-24 cursor-pointer hover:bg-slate-700/50 hover:text-white transition-colors"
                  >
                    <div className="flex items-center gap-1">
                      <span>Gametype</span>
                      {sortColumn === 'type' && <span className="text-sky-400 font-bold">{sortDirection === 'asc' ? '▲' : '▼'}</span>}
                    </div>
                  </th>

                  {/* Clickable IP Column */}
                  <th 
                    onClick={() => handleSort('ip')} 
                    className="py-1.5 px-2 w-36 cursor-pointer hover:bg-slate-700/50 hover:text-white transition-colors"
                  >
                    <div className="flex items-center gap-1">
                      <span>IP Address : Port</span>
                      {sortColumn === 'ip' && <span className="text-sky-400 font-bold">{sortDirection === 'asc' ? '▲' : '▼'}</span>}
                    </div>
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/70 text-slate-200 font-mono text-[11px]">
                {isLoading ? (
                  <tr>
                    <td colSpan={9} className="py-12 text-center text-slate-400 font-mono">
                      <div className="inline-block size-6 border-2 border-sky-400 border-t-transparent rounded-full animate-spin mb-2" />
                      <div>Querying master mirror cache...</div>
                    </td>
                  </tr>
                ) : filteredServers.length === 0 ? (
                  <tr>
                    <td colSpan={9} className="py-12 text-center text-slate-400 font-mono">
                      <div>No servers match the selected tree sector or filter criteria.</div>
                    </td>
                  </tr>
                ) : (
                  filteredServers.map((srv) => {
                    const isSelected = selectedServer?.id === srv.id;
                    const isFav = favorites.has(srv.id);
                    const { flag } = getCountryFlag(srv.ip);
                    const cleanName = sanitizeText(srv.hostname) || `${srv.ip}:${srv.gamePort || srv.queryPort}`;
                    const cleanMap = sanitizeText(srv.map) || '---';
                    const pingColor = srv.ping <= 50 ? 'text-emerald-400 font-bold' : srv.ping <= 100 ? 'text-yellow-400' : 'text-rose-400';
                    const maxP = srv.maxPlayers || 32;

                    return (
                      <tr
                        key={srv.id}
                        onClick={() => handleSelectServer(srv)}
                        onDoubleClick={() => handleConnect(srv)}
                        className={`cursor-pointer transition-colors ${
                          isSelected
                            ? 'ase-grid-row-selected'
                            : 'hover:bg-sky-950/40 odd:bg-[#111620] even:bg-[#0c1017]'
                        }`}
                        title="Click to select, Double-click to connect"
                      >
                        {/* Star */}
                        <td 
                          className="py-1 px-1.5 border-r border-slate-800/80 text-center"
                          onClick={(e) => toggleFavorite(srv.id, e)}
                        >
                          <Star className={`size-3 mx-auto ${isFav ? 'fill-amber-400 text-amber-400' : 'text-slate-600 hover:text-amber-400'}`} />
                        </td>

                        {/* Country Flag */}
                        <td className="py-1 px-1.5 border-r border-slate-800/80 text-center text-xs">
                          {flag}
                        </td>

                        {/* Game tag */}
                        <td className="py-1 px-2 border-r border-slate-800/80 uppercase text-[10px] text-sky-400 font-bold truncate">
                          {srv.gamename}
                        </td>

                        {/* Server Name */}
                        <td className="py-1 px-2 border-r border-slate-800/80 font-semibold truncate max-w-[260px]" title={cleanName}>
                          {cleanName}
                        </td>

                        {/* Ping */}
                        <td className={`py-1 px-2 border-r border-slate-800/80 text-right ${pingColor}`}>
                          {srv.ping}ms
                        </td>

                        {/* Players */}
                        <td className="py-1 px-2 border-r border-slate-800/80 text-center font-bold text-amber-300">
                          {srv.players}/{maxP}
                        </td>

                        {/* Map */}
                        <td className="py-1 px-2 border-r border-slate-800/80 text-slate-300 truncate max-w-[120px]" title={cleanMap}>
                          {cleanMap}
                        </td>

                        {/* Gametype */}
                        <td className="py-1 px-2 border-r border-slate-800/80 text-slate-400 truncate">
                          {srv.gametype || 'Standard'}
                        </td>

                        {/* IP:Port */}
                        <td className="py-1 px-2 text-slate-400 font-mono text-[10px]">
                          {srv.ip}:{srv.gamePort || srv.queryPort}
                        </td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>

          {/* Bottom Half: The Classic ASE Dual Inspector (Full-Height Player Roster + Server Telemetry & Dropdown CVars) */}
          <div className="h-[260px] shrink-0 grid grid-cols-1 md:grid-cols-2 divide-y md:divide-y-0 md:divide-x divide-slate-800 bg-[#10141c] p-2.5 text-xs overflow-hidden">
            
            {/* Player Roster Panel - Fully Fills Its Area */}
            <div className="flex flex-col h-full space-y-2 pr-0 md:pr-3 overflow-hidden">
              <div className="flex items-center justify-between text-[11px] font-bold text-sky-400 border-b border-slate-700/80 pb-1.5 shrink-0">
                <span className="flex items-center gap-1.5">
                  <Users className="size-3.5 text-sky-400" />
                  <span>Combat Roster ({selectedServer?.roster?.length || selectedServer?.players || 0} Players)</span>
                </span>
                <span className="text-slate-400 text-[10px] font-mono">Score // Ping</span>
              </div>

              {selectedServer?.roster && selectedServer.roster.length > 0 ? (
                <div className="flex-1 overflow-y-auto space-y-1 font-mono text-[11px] text-slate-300 pr-1.5 scrollbar-thin">
                  {selectedServer.roster.map((player, idx) => (
                    <div 
                      key={idx}
                      className="flex items-center justify-between px-2.5 py-1 rounded bg-slate-900/60 hover:bg-slate-850 border border-slate-800/40"
                    >
                      <span className="text-slate-200 truncate max-w-[200px] font-medium">
                        {sanitizeText(player.name) || 'Player'}
                      </span>
                      <span className="text-emerald-400 font-bold shrink-0 font-mono">
                        {player.frags} pts <span className="text-slate-500 font-normal">({player.ping}ms)</span>
                      </span>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="flex-1 flex flex-col items-center justify-center text-slate-500 text-xs italic py-8 text-center font-mono">
                  <Users className="size-6 text-slate-700 mb-1" />
                  <div>No active player roster data reported by server query.</div>
                </div>
              )}
            </div>

            {/* Server CVars & Info Panel - Clean Core Telemetry + Collapsible CVars Dropdown */}
            <div className="flex flex-col h-full space-y-2 pt-2 md:pt-0 pl-0 md:pl-3 overflow-hidden justify-between">
              <div className="flex flex-col flex-1 min-h-0 space-y-2">
                <div className="flex items-center justify-between text-[11px] font-bold text-sky-400 border-b border-slate-700/80 pb-1.5 shrink-0">
                  <span className="flex items-center gap-1.5">
                    <ServerIcon className="size-3.5 text-sky-400" />
                    <span>Server Telemetry &amp; Rules</span>
                  </span>
                  <span className="text-slate-400 font-mono text-[10px] bg-slate-900 px-1.5 py-0.5 rounded border border-slate-800">
                    {selectedServer ? `${selectedServer.ip}:${selectedServer.gamePort || selectedServer.queryPort}` : '---'}
                  </span>
                </div>

                {selectedServer ? (
                  <div className="flex-1 overflow-y-auto space-y-2 font-mono text-[11px] text-slate-300 pr-1.5 scrollbar-thin">
                    {/* Core Highlighted Server Properties */}
                    <div className="grid grid-cols-2 gap-1.5 shrink-0">
                      <div className="p-2 rounded bg-slate-900/80 border border-slate-800">
                        <div className="text-[10px] text-slate-500 uppercase">Gamename &amp; Mode</div>
                        <div className="text-sky-300 font-bold truncate mt-0.5">
                          {selectedServer.gamename} {selectedServer.gametype ? `(${selectedServer.gametype})` : ''}
                        </div>
                      </div>
                      <div className="p-2 rounded bg-slate-900/80 border border-slate-800">
                        <div className="text-[10px] text-slate-500 uppercase">Map Roster</div>
                        <div className="text-amber-300 font-bold truncate mt-0.5">{selectedServer.map}</div>
                      </div>
                      <div className="p-2 rounded bg-slate-900/80 border border-slate-800">
                        <div className="text-[10px] text-slate-500 uppercase">Player Capacity</div>
                        <div className="text-emerald-400 font-bold mt-0.5">{selectedServer.players} / {selectedServer.maxPlayers} Active</div>
                      </div>
                      <div className="p-2 rounded bg-slate-900/80 border border-slate-800">
                        <div className="text-[10px] text-slate-500 uppercase">Latency Roundtrip</div>
                        <div className="text-slate-200 font-bold mt-0.5">{selectedServer.ping}ms Ping</div>
                      </div>
                    </div>

                    {/* Additional Metadata Strip */}
                    <div className="p-2 rounded bg-slate-900/40 border border-slate-800/80 text-[10px] space-y-1">
                      <div className="flex justify-between">
                        <span className="text-slate-500">ENGINE PROTOCOL:</span>
                        <span className="text-sky-400 font-semibold uppercase">{selectedServer.gamename.includes('cod') || selectedServer.gamename.includes('q3') ? 'Quake 3 / idTech 3' : selectedServer.gamename.includes('q2') || selectedServer.gamename === 'kingpin' ? 'Quake 2 / idTech 2' : 'GameSpy Master'}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-slate-500">QUERY PORT / GAME PORT:</span>
                        <span className="text-slate-300 font-mono">{selectedServer.queryPort} / {selectedServer.gamePort}</span>
                      </div>
                    </div>

                    {/* Server CVars Dropdown (Hidden by default, expands on click) */}
                    {selectedServer.cvars && Object.keys(selectedServer.cvars).length > 0 ? (
                      <div className="border border-slate-800 rounded-lg bg-slate-900/60 overflow-hidden">
                        <button
                          type="button"
                          onClick={() => {
                            sfx.playClick();
                            setCvarsOpen(!cvarsOpen);
                          }}
                          className="w-full px-2.5 py-1.5 flex items-center justify-between text-[11px] font-mono text-sky-300 hover:text-white hover:bg-slate-800/60 transition-colors"
                        >
                          <span className="flex items-center gap-1.5 font-semibold">
                            <Sliders className="size-3 text-sky-400" />
                            <span>Server CVars &amp; Configuration ({Object.keys(selectedServer.cvars).length} variables)</span>
                          </span>
                          <span className="text-[10px] text-slate-400 flex items-center gap-1">
                            <span>{cvarsOpen ? 'Collapse ▲' : 'Expand Dropdown ▼'}</span>
                          </span>
                        </button>

                        {cvarsOpen && (
                          <div className="p-2 border-t border-slate-800 max-h-56 overflow-y-auto space-y-1 font-mono text-[10px] scrollbar-thin bg-slate-950/80">
                            {Object.entries(selectedServer.cvars).map(([k, v]) => (
                              <div 
                                key={k} 
                                className="flex items-center justify-between px-2 py-0.5 rounded bg-slate-900/60 hover:bg-slate-800/60 text-[10px] border border-slate-800/40"
                              >
                                <span className="text-sky-400 font-semibold">{k}:</span>
                                <span className="text-slate-200 truncate max-w-[220px]" title={v}>{v}</span>
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    ) : (
                      <div className="text-slate-500 italic py-1 text-center text-[10px]">No extended CVars declared.</div>
                    )}
                  </div>
                ) : (
                  <div className="flex-1 flex items-center justify-center text-slate-500 italic py-4 text-center font-mono">
                    Select a server to view rules and telemetry.
                  </div>
                )}
              </div>

              {/* Quick Connect & Multi-Copy Toolbar */}
              {selectedServer && (
                <div className="pt-2 border-t border-slate-800/80 flex flex-col sm:flex-row sm:items-center justify-between gap-2 shrink-0">
                  <div className="text-[10px] font-mono text-slate-400 truncate max-w-[260px]">
                    <span>Target: </span>
                    <code className="text-sky-300 bg-slate-900 px-1 py-0.5 rounded">
                      {selectedServer.ip}:{selectedServer.gamePort || selectedServer.queryPort}
                    </code>
                  </div>

                  <div className="flex items-center gap-1.5 self-end sm:self-auto">
                    {/* Raw IP copy */}
                    <button
                      onClick={() => handleCopyRawIP(selectedServer)}
                      className="px-2 py-1 rounded bg-slate-900 border border-slate-700 text-slate-300 text-[10px] hover:border-slate-500 transition-colors"
                      title="Copy raw ip:port address"
                    >
                      {copiedRawId === selectedServer.id ? 'IP COPIED!' : 'COPY IP'}
                    </button>

                    {/* Console /connect copy */}
                    <button
                      onClick={() => handleConnect(selectedServer)}
                      className="px-2.5 py-1 rounded bg-sky-950 border border-sky-500/40 text-sky-300 font-bold text-[11px] hover:bg-sky-900 transition-colors"
                      title="Copy /connect console command"
                    >
                      {copiedId === selectedServer.id ? 'COPIED!' : 'COPY /connect'}
                    </button>

                    {/* Steam Direct Launch if supported */}
                    {['cod', 'cod2', 'cod4', 'et', 'rtcw', 'q3a', 'quake3', 'quake2'].includes(selectedServer.gamename.toLowerCase()) && (
                      <a
                        href={`steam://connect/${selectedServer.ip}:${selectedServer.gamePort || selectedServer.queryPort}`}
                        className="px-2 py-1 rounded bg-slate-800 border border-slate-600 text-slate-300 text-[10px] hover:text-white hover:border-sky-400 transition-colors flex items-center gap-1"
                        title="Launch game via Steam protocol"
                      >
                        <span>STEAM</span>
                        <ExternalLink className="size-2.5" />
                      </a>
                    )}
                  </div>
                </div>
              )}
            </div>

          </div>

        </div>
      </div>

      {/* =========================================================================
          BOTTOM ASE STATUS BAR (SPANS FULL WINDOW)
         ========================================================================= */}
      <div className="shrink-0 ase-menubar px-3 py-1 flex items-center justify-between text-[10px] text-slate-400 font-mono border-t border-slate-800 select-none z-10">
        <div className="truncate">
          <span>Status: Ready</span> | <span>Selected: {selectedServer ? sanitizeText(selectedServer.hostname) : 'None'}</span>
        </div>
        <div className="shrink-0 flex items-center gap-3">
          <span>Master: retro-server-mirror [ONLINE]</span>
          <span>{filteredServers.length} / {data?.totalServers || 0} servers listed</span>
        </div>
      </div>

      {/* =========================================================================
          CATALOG DOSSIER DRAWER (4,678 GAMES DIRECTORY)
         ========================================================================= */}
      <Sheet open={catalogOpen} onOpenChange={setCatalogOpen}>
        <SheetContent 
          side="right" 
          className="w-full sm:max-w-xl bg-slate-950 border-l border-sky-500/30 text-slate-100 p-0 flex flex-col font-mono"
        >
          {/* Drawer Header */}
          <SheetHeader className="p-4 border-b border-sky-500/20 bg-slate-900/60">
            <div className="flex items-center gap-2">
              <Database className="size-5 text-sky-400" />
              <SheetTitle className="font-mono text-base font-bold text-sky-300 uppercase tracking-wider">
                All-Seeing Eye Game Catalog (4,678 Titles)
              </SheetTitle>
            </div>
            <p className="text-xs text-slate-400 mt-1">
              Search and filter across all 4,678 cataloged GameSpy, Quake, and Unreal multiplayer games.
            </p>
          </SheetHeader>

          {/* Drawer Controls */}
          <div className="p-4 border-b border-slate-800 space-y-3 bg-slate-950">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-sky-400/60" />
              <input
                type="text"
                value={catalogSearch}
                onChange={(e) => setCatalogSearch(e.target.value)}
                placeholder="SEARCH TITLES (E.G. DOOM, QUAKE, COD, BATTLEFIELD)..."
                className="w-full pl-9 pr-4 py-2 rounded-lg bg-slate-900 border border-slate-800 focus:border-sky-400 focus:outline-none text-slate-200 text-xs font-mono"
              />
            </div>

            {/* Categories */}
            <div className="flex items-center gap-1.5 overflow-x-auto pb-1 text-[11px]">
              {['all', 'GameSpy PC Classic', 'idTech / Quake', 'EA / DICE', 'Nintendo DWC', 'Unreal Engine'].map((cat) => (
                <button
                  key={cat}
                  onClick={() => {
                    sfx.playClick();
                    setCatalogCategory(cat);
                  }}
                  className={`px-2 py-1 rounded whitespace-nowrap border transition-all ${
                    catalogCategory === cat
                      ? 'bg-sky-500/20 border-sky-400 text-sky-300 font-bold'
                      : 'bg-slate-900 border-slate-800 text-slate-400 hover:text-slate-200'
                  }`}
                >
                  {cat === 'all' ? 'All Sectors' : cat}
                </button>
              ))}
            </div>
          </div>

          {/* Game List */}
          <div className="flex-1 overflow-y-auto p-4 space-y-2">
            {catalogData?.games?.map((g) => {
              const icon = getGameIcon(g.gamename);
              const isSelected = treeSelection.toLowerCase() === g.gamename.toLowerCase();

              return (
                <div
                  key={g.gamename}
                  className={`p-2.5 rounded-lg border transition-all flex items-center justify-between gap-3 ${
                    isSelected
                      ? 'bg-sky-950/60 border-sky-400 text-sky-200'
                      : 'bg-slate-900/50 border-slate-800/80 hover:border-slate-700'
                  }`}
                >
                  <div className="flex items-center gap-2.5 min-w-0">
                    <div className="relative size-9 rounded border border-slate-800 bg-slate-950 overflow-hidden shrink-0">
                      {icon ? (
                        <Image src={icon} alt={g.label} fill sizes="36px" className="object-cover" />
                      ) : (
                        <Gamepad2 className="size-5 m-2 text-slate-600" />
                      )}
                    </div>
                    <div className="min-w-0">
                      <div className="font-bold text-xs text-slate-100 truncate">{g.label}</div>
                      <div className="flex items-center gap-1.5 text-[10px] text-slate-500 mt-0.5">
                        <span>{g.gamename}</span>
                        <span>•</span>
                        <span className="text-sky-400/80">{g.category || 'Classic'}</span>
                      </div>
                    </div>
                  </div>

                  <button
                    onClick={() => {
                      sfx.playDeploy();
                      setTreeSelection(g.gamename);
                      setCatalogOpen(false);
                    }}
                    className="px-2.5 py-1 rounded text-[11px] font-mono bg-sky-950 border border-sky-500/30 text-sky-300 hover:bg-sky-900 hover:border-sky-400 transition-colors shrink-0"
                  >
                    SELECT
                  </button>
                </div>
              );
            })}
          </div>
        </SheetContent>
      </Sheet>

    </div>
  );
}
