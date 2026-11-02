'use client';

import React, { useState, useMemo, useEffect } from 'react';
import useSWR from 'swr';
import Image from 'next/image';
import { 
  Server as ServerIcon, 
  Users, 
  Search, 
  Copy, 
  Play, 
  Gamepad2,
  Database,
  Volume2,
  VolumeX,
  Star,
  Zap,
  Globe,
  Folder,
  FolderOpen,
  X,
  Sliders,
  History,
  ExternalLink,
  Sun,
  Moon
} from 'lucide-react';
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

type SortColumn = 'ping' | 'players' | 'game' | 'name' | 'map' | 'type' | 'ip' | 'score';

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
    { refreshInterval: 10000, revalidateOnFocus: true }
  );

  // Theme: Default to Authentic Classic Win32 ASE
  const [theme, setTheme] = useState<'classic' | 'dark'>('classic');

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
  const [hideFull, setHideFull] = useState<boolean>(false);
  const [serverTypeFilter, setServerTypeFilter] = useState<string>('all');

  // Sorting
  const [sortColumn, setSortColumn] = useState<SortColumn>('players');
  const [sortDirection, setSortDirection] = useState<'asc' | 'desc'>('desc');

  // Tree category toggles
  const [showGamesFolder, setShowGamesFolder] = useState<boolean>(true);
  const [showRegionsFolder, setShowRegionsFolder] = useState<boolean>(true);
  const [showServerTypeFolder, setShowServerTypeFolder] = useState<boolean>(true);

  // Bottom Left Sub-Tab (Buddy vs Server)
  const [leftSubTab, setLeftSubTab] = useState<'buddy' | 'server'>('server');

  // Bottom Right Sub-Tab (Players vs Rules)
  const [rightSubTab, setRightSubTab] = useState<'players' | 'rules'>('players');

  // Catalog Drawer
  const [catalogOpen, setCatalogOpen] = useState<boolean>(false);
  const [catalogSearch, setCatalogSearch] = useState<string>('');
  const [catalogCategory, setCatalogCategory] = useState<string>('all');

  // Load favorites & history & theme from localStorage
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
      const savedTheme = localStorage.getItem('ase_theme');
      if (savedTheme === 'dark' || savedTheme === 'classic') {
        setTheme(savedTheme);
      }
    } catch {
      // LocalStorage fallback
    }
  }, []);

  const toggleFavorite = (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
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

  const toggleTheme = () => {
    const next = theme === 'classic' ? 'dark' : 'classic';
    setTheme(next);
    try {
      localStorage.setItem('ase_theme', next);
    } catch {}
    sfx.playClick(1200);
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

      // Hide full
      if (hideFull && srv.players >= (srv.maxPlayers || 32)) return false;

      // Server Type filter
      if (serverTypeFilter === 'dedicated' && srv.cvars && srv.cvars['dedicated'] === '0') return false;
      if (serverTypeFilter === 'linux' && srv.cvars && !srv.cvars['version']?.toLowerCase().includes('linux')) return false;
      if (serverTypeFilter === 'win' && srv.cvars && !srv.cvars['version']?.toLowerCase().includes('win')) return false;

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
      const nameKey = cleanHost ? `${gName}:${srv.ip}:${cleanHost}` : '';

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
  }, [data?.servers, treeSelection, regionFilter, hideEmpty, hideFull, serverTypeFilter, searchQuery, sortColumn, sortDirection, favorites, history]);

  // Ensure an active selected server
  const selectedServer = useMemo(() => {
    if (!filteredServers || filteredServers.length === 0) return null;
    if (selectedServerId) {
      const match = filteredServers.find((s) => s.id === selectedServerId);
      if (match) return match;
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

  const isClassic = theme === 'classic';

  return (
    <div className={`h-screen max-h-screen h-dvh max-h-dvh flex flex-col overflow-hidden select-none ${
      isClassic ? 'ase-classic' : 'bg-[#0d1117] text-slate-100 font-sans'
    }`}>

      {/* =========================================================================
          1. CLASSIC WINDOWS XP / 2000 TITLEBAR
         ========================================================================= */}
      <div className={`shrink-0 flex items-center justify-between px-2 py-0.5 border-b select-none ${
        isClassic ? 'ase-titlebar border-[#716f64]' : 'bg-[#19202a] text-slate-200 border-slate-800'
      }`}>
        <div className="flex items-center gap-1.5 font-bold">
          <div className="relative size-4 rounded overflow-hidden shrink-0">
            <Image 
              src="/ase-logo.png" 
              alt="The All-Seeing Eye Logo" 
              fill 
              sizes="16px" 
              className="object-cover" 
            />
          </div>
          <span>The All-Seeing Eye - default</span>
        </div>

        {/* Windows Control Buttons (Minimize, Maximize, Close) */}
        <div className="flex items-center gap-0.5">
          <button 
            type="button" 
            className="w-4 h-3.5 bg-[#ece9d8] text-black border border-white border-r-[#716f64] border-b-[#716f64] flex items-center justify-center text-[9px] font-bold leading-none cursor-default"
          >
            _
          </button>
          <button 
            type="button" 
            className="w-4 h-3.5 bg-[#ece9d8] text-black border border-white border-r-[#716f64] border-b-[#716f64] flex items-center justify-center text-[10px] font-bold leading-none cursor-default"
          >
            □
          </button>
          <button 
            type="button" 
            className="w-4 h-3.5 bg-[#ece9d8] text-black border border-white border-r-[#716f64] border-b-[#716f64] flex items-center justify-center text-[10px] font-bold leading-none cursor-default hover:bg-[#c75050] hover:text-white"
          >
            ✕
          </button>
        </div>
      </div>

      {/* =========================================================================
          2. AUTHENTIC WIN32 MENU BAR
         ========================================================================= */}
      <div className={`shrink-0 px-2 py-0.5 flex items-center gap-3 text-[11px] border-b ${
        isClassic ? 'bg-[#ece9d8] border-[#716f64] text-black' : 'bg-[#131822] border-slate-800 text-slate-300 font-mono'
      }`}>
        {['File', 'View', 'Filter', 'Server', 'Player', 'Buddy', 'Channels', 'Tools', 'Community', 'Help'].map((menu) => (
          <span 
            key={menu} 
            className={`px-1.5 py-0.2 cursor-pointer transition-colors ${
              isClassic ? 'hover:bg-[#0a246a] hover:text-white' : 'hover:text-white hover:bg-slate-800'
            }`}
          >
            {menu}
          </span>
        ))}
      </div>

      {/* =========================================================================
          3. AUTHENTIC 3D RAISED TOOLBAR
         ========================================================================= */}
      <div className={`shrink-0 p-1 flex items-center flex-wrap gap-1.5 border-b text-[11px] ${
        isClassic ? 'ase-raised border-b-[#716f64]' : 'bg-[#171d27] border-slate-800'
      }`}>
        {/* Refresh Master Button */}
        <button 
          onClick={() => {
            sfx.playSweep();
            mutate();
          }}
          className={`px-2 py-0.5 flex items-center gap-1.5 cursor-pointer ${
            isClassic ? 'ase-win32-btn' : 'bg-slate-800 border border-slate-700 text-white rounded'
          }`}
          title="Refresh servers from master (F5)"
        >
          <span className="text-xs">👁️</span>
          <span className="font-semibold">Refresh</span>
        </button>

        {/* Stop Button */}
        <button 
          onClick={() => sfx.playClick(800)}
          className={`px-2 py-0.5 flex items-center gap-1 cursor-pointer ${
            isClassic ? 'ase-win32-btn' : 'bg-slate-800 border border-slate-700 text-slate-300 rounded'
          }`}
          title="Stop scanning"
        >
          <span className="text-red-600 font-bold">■</span>
          <span>Stop</span>
        </button>

        {/* Ping Visible Button */}
        <button 
          onClick={() => {
            sfx.playClick(1200);
            mutate();
          }}
          className={`px-2 py-0.5 flex items-center gap-1 cursor-pointer ${
            isClassic ? 'ase-win32-btn' : 'bg-slate-800 border border-slate-700 text-yellow-300 rounded'
          }`}
          title="Ping visible servers"
        >
          <Zap className="size-3 text-amber-500 fill-amber-500" />
          <span>Ping</span>
        </button>

        {/* Quick Filter Search Input */}
        <div className="flex items-center gap-1 ml-1">
          <span className="text-slate-600 font-semibold text-[10px]">Filter:</span>
          <div className={`flex items-center px-1.5 py-0.5 ${isClassic ? 'ase-sunken' : 'bg-slate-900 border border-slate-700 rounded'}`}>
            <Search className="size-3 text-slate-500 mr-1" />
            <input 
              id="ase-filter-search"
              type="text" 
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search (Press '/' to focus)..." 
              className="bg-transparent text-[11px] focus:outline-none w-36 sm:w-48 placeholder:text-slate-400"
            />
            {searchQuery && (
              <button onClick={() => setSearchQuery('')} className="text-slate-500 hover:text-black">
                <X className="size-2.5" />
              </button>
            )}
          </div>
        </div>

        {/* Not Empty Filter Toggle */}
        <button
          onClick={() => {
            sfx.playClick();
            setHideEmpty(!hideEmpty);
          }}
          className={`px-2 py-0.5 flex items-center gap-1 cursor-pointer ${
            hideEmpty
              ? isClassic ? 'ase-win32-btn-active font-bold text-blue-900' : 'bg-sky-500/20 border border-sky-400 text-sky-300 font-bold rounded'
              : isClassic ? 'ase-win32-btn text-slate-700' : 'bg-slate-800 border border-slate-700 text-slate-300 rounded'
          }`}
          title="Hide empty servers"
        >
          <span>[ Not Empty ]</span>
        </button>

        {/* Not Full Filter Toggle */}
        <button
          onClick={() => {
            sfx.playClick();
            setHideFull(!hideFull);
          }}
          className={`px-2 py-0.5 flex items-center gap-1 cursor-pointer ${
            hideFull
              ? isClassic ? 'ase-win32-btn-active font-bold text-blue-900' : 'bg-sky-500/20 border border-sky-400 text-sky-300 font-bold rounded'
              : isClassic ? 'ase-win32-btn text-slate-700' : 'bg-slate-800 border border-slate-700 text-slate-300 rounded'
          }`}
          title="Hide full servers"
        >
          <span>[ Not Full ]</span>
        </button>

        {/* Connect to Game Button */}
        {selectedServer && (
          <button 
            onClick={() => handleConnect(selectedServer)}
            className={`px-2.5 py-0.5 font-bold cursor-pointer transition-colors flex items-center gap-1 ${
              isClassic ? 'ase-win32-btn text-emerald-800' : 'bg-emerald-950 border border-emerald-500/40 text-emerald-300 rounded'
            }`}
            title="Copy /connect IP:PORT"
          >
            <Play className="size-3 fill-emerald-600 text-emerald-600" />
            <span>{copiedId === selectedServer.id ? 'COPIED!' : 'CONNECT'}</span>
          </button>
        )}

        {/* Right side tools */}
        <div className="ml-auto flex items-center gap-1.5">
          {/* SFX Audio Button */}
          <button
            onClick={toggleAudio}
            className={`px-1.5 py-0.5 flex items-center gap-1 cursor-pointer ${
              isClassic ? 'ase-win32-btn' : 'bg-slate-800 border border-slate-700 rounded'
            }`}
            title="Toggle synthesized Web Audio FX"
          >
            {audioEnabled ? <Volume2 className="size-3 text-sky-600" /> : <VolumeX className="size-3 text-slate-400" />}
            <span className="text-[10px]">SFX</span>
          </button>

          {/* Theme Switcher Toggle */}
          <button
            onClick={toggleTheme}
            className={`px-2 py-0.5 flex items-center gap-1 cursor-pointer text-[10px] font-semibold ${
              isClassic ? 'ase-win32-btn' : 'bg-slate-800 border border-slate-700 rounded text-slate-200'
            }`}
            title="Switch between Authentic Classic 2002 Win32 theme and Modern Dark mode"
          >
            {isClassic ? <Moon className="size-3 text-slate-600" /> : <Sun className="size-3 text-yellow-400" />}
            <span>{isClassic ? 'Classic 2002' : 'Modern Dark'}</span>
          </button>

          {/* 4,678 Games Catalog Drawer Button */}
          <button
            onClick={() => {
              sfx.playDeploy();
              setCatalogOpen(true);
            }}
            className={`px-2 py-0.5 flex items-center gap-1 font-semibold cursor-pointer ${
              isClassic ? 'ase-win32-btn' : 'bg-slate-800 border border-slate-700 rounded text-sky-300'
            }`}
          >
            <Database className="size-3 text-sky-600" />
            <span>Catalog ▼</span>
          </button>
        </div>
      </div>

      {/* =========================================================================
          4. MAIN APPLICATION BODY (SPLIT VIEW)
         ========================================================================= */}
      <div className={`flex-1 min-h-0 grid grid-cols-1 md:grid-cols-12 gap-1 p-1 overflow-hidden ${
        isClassic ? 'bg-[#ece9d8]' : 'bg-[#0d1117]'
      }`}>

        {/* ---------------------------------------------------------------------
            LEFT PANE: AUTHENTIC DIRECTORY TREE & BOTTOM BUDDY/SERVER TAB (3 COLS)
           --------------------------------------------------------------------- */}
        <div className="md:col-span-3 h-full flex flex-col min-h-0 overflow-hidden">
          
          {/* Top Half: Windows Explorer Style Collapsible Tree */}
          <div className={`flex-1 min-h-0 overflow-y-auto p-1.5 scrollbar-thin ${
            isClassic ? 'ase-sunken' : 'bg-[#131822] border border-slate-800 rounded'
          }`}>
            
            {/* Tree Root: Scanner Categories */}
            <div className="mb-2">
              <div className="font-bold flex items-center gap-1 mb-1 text-slate-800 cursor-pointer">
                <span className="ase-tree-expander">-</span>
                <FolderOpen className="size-3.5 text-amber-500 fill-amber-500" />
                <span className="font-bold">Scanner Categories</span>
              </div>

              <div className="pl-4 space-y-0.5">
                {/* All Servers */}
                <div 
                  onClick={() => {
                    sfx.playClick();
                    setTreeSelection('all');
                    setRegionFilter('all');
                    setServerTypeFilter('all');
                  }}
                  className={`px-1.5 py-0.5 rounded cursor-pointer flex items-center justify-between transition-colors ${
                    treeSelection === 'all' && regionFilter === 'all' && serverTypeFilter === 'all'
                      ? 'ase-row-selected' 
                      : 'hover:bg-[#316ac5] hover:text-white'
                  }`}
                >
                  <div className="flex items-center gap-1 truncate">
                    <Folder className="size-3 text-amber-500 fill-amber-500" />
                    <span>All Active Servers</span>
                  </div>
                  <span className="font-bold text-[10px]">
                    {data?.totalServers || 0}
                  </span>
                </div>

                {/* Favorites */}
                <div 
                  onClick={() => {
                    sfx.playClick();
                    setTreeSelection('favorites');
                  }}
                  className={`px-1.5 py-0.5 rounded cursor-pointer flex items-center justify-between transition-colors ${
                    treeSelection === 'favorites' 
                      ? 'ase-row-selected' 
                      : 'hover:bg-[#316ac5] hover:text-white'
                  }`}
                >
                  <div className="flex items-center gap-1 truncate">
                    <Star className="size-3 text-amber-500 fill-amber-500" />
                    <span>Favorites List</span>
                  </div>
                  <span className="font-bold text-[10px]">
                    {favorites.size}
                  </span>
                </div>

                {/* History */}
                <div 
                  onClick={() => {
                    sfx.playClick();
                    setTreeSelection('history');
                  }}
                  className={`px-1.5 py-0.5 rounded cursor-pointer flex items-center justify-between transition-colors ${
                    treeSelection === 'history' 
                      ? 'ase-row-selected' 
                      : 'hover:bg-[#316ac5] hover:text-white'
                  }`}
                >
                  <div className="flex items-center gap-1 truncate">
                    <History className="size-3 text-slate-500" />
                    <span>Recent History</span>
                  </div>
                  <span className="font-bold text-[10px]">
                    {history.length}
                  </span>
                </div>
              </div>
            </div>

            {/* Tree Branch: Active Games */}
            <div className="mb-2">
              <div 
                onClick={() => setShowGamesFolder(!showGamesFolder)}
                className="font-bold flex items-center justify-between mb-1 text-slate-800 cursor-pointer"
              >
                <div className="flex items-center gap-1">
                  <span className="ase-tree-expander">{showGamesFolder ? '-' : '+'}</span>
                  <FolderOpen className="size-3.5 text-amber-500 fill-amber-500" />
                  <span>Games ({activeGameDistribution.length})</span>
                </div>
              </div>

              {showGamesFolder && (
                <div className="pl-4 space-y-0.5">
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
                        className={`px-1.5 py-0.5 rounded cursor-pointer flex items-center justify-between transition-colors ${
                          isSelected 
                            ? 'ase-row-selected' 
                            : 'hover:bg-[#316ac5] hover:text-white'
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
                        <span className="font-bold text-[10px] shrink-0">
                          {game.serverCount}
                        </span>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>

            {/* Tree Branch: By Territory */}
            <div className="mb-2">
              <div 
                onClick={() => setShowRegionsFolder(!showRegionsFolder)}
                className="font-bold flex items-center justify-between mb-1 text-slate-800 cursor-pointer"
              >
                <div className="flex items-center gap-1">
                  <span className="ase-tree-expander">{showRegionsFolder ? '-' : '+'}</span>
                  <FolderOpen className="size-3.5 text-amber-500 fill-amber-500" />
                  <span>By Territory</span>
                </div>
              </div>

              {showRegionsFolder && (
                <div className="pl-4 space-y-0.5">
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
                      className={`px-1.5 py-0.5 rounded cursor-pointer flex items-center justify-between transition-colors ${
                        regionFilter === reg.name 
                          ? 'ase-row-selected' 
                          : 'hover:bg-[#316ac5] hover:text-white'
                      }`}
                    >
                      <div className="flex items-center gap-1.5 truncate">
                        <span>{reg.flag}</span>
                        <span>{reg.name}</span>
                      </div>
                      {regionFilter === reg.name && (
                        <span className="text-[9px] font-bold">ACTIVE</span>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* Tree Branch: Server Type & Status */}
            <div>
              <div 
                onClick={() => setShowServerTypeFolder(!showServerTypeFolder)}
                className="font-bold flex items-center justify-between mb-1 text-slate-800 cursor-pointer"
              >
                <div className="flex items-center gap-1">
                  <span className="ase-tree-expander">{showServerTypeFolder ? '-' : '+'}</span>
                  <FolderOpen className="size-3.5 text-amber-500 fill-amber-500" />
                  <span>Server Type</span>
                </div>
              </div>

              {showServerTypeFolder && (
                <div className="pl-4 space-y-0.5 text-[11px]">
                  {[
                    { key: 'all', label: 'All Server Types' },
                    { key: 'dedicated', label: 'Dedicated Only' },
                    { key: 'linux', label: 'Linux OS' },
                    { key: 'win', label: 'Windows OS' },
                  ].map((st) => (
                    <div
                      key={st.key}
                      onClick={() => {
                        sfx.playClick();
                        setServerTypeFilter(st.key);
                      }}
                      className={`px-1.5 py-0.5 rounded cursor-pointer flex items-center gap-1.5 transition-colors ${
                        serverTypeFilter === st.key ? 'ase-row-selected' : 'hover:bg-[#316ac5] hover:text-white'
                      }`}
                    >
                      <input 
                        type="checkbox" 
                        checked={serverTypeFilter === st.key} 
                        readOnly 
                        className="size-3 pointer-events-none" 
                      />
                      <span>{st.label}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>

          </div>

          {/* Bottom Half: Authentic Buddy / Server Inspector Tabs */}
          <div className="h-[155px] shrink-0 flex flex-col mt-1 overflow-hidden">
            <div className="flex items-center pl-1 z-10">
              <button
                onClick={() => setLeftSubTab('server')}
                className={`ase-tab ${leftSubTab === 'server' ? 'ase-tab-active' : ''}`}
              >
                Server
              </button>
              <button
                onClick={() => setLeftSubTab('buddy')}
                className={`ase-tab ${leftSubTab === 'buddy' ? 'ase-tab-active' : ''}`}
              >
                Buddy
              </button>
            </div>

            <div className={`flex-1 min-h-0 p-1.5 overflow-y-auto ${
              isClassic ? 'ase-sunken' : 'bg-[#131822] border border-slate-800'
            }`}>
              {leftSubTab === 'server' ? (
                selectedServer ? (
                  <div className="space-y-1 text-[11px]">
                    <div className="font-bold truncate text-[#0a246a]" title={selectedServer.hostname}>
                      {sanitizeText(selectedServer.hostname) || selectedServer.ip}
                    </div>
                    <div className="text-[10px] text-slate-600 font-mono">
                      {selectedServer.ip}:{selectedServer.gamePort || selectedServer.queryPort}
                    </div>
                    <div className="grid grid-cols-2 gap-1 text-[10px]">
                      <div>Game: <span className="font-bold">{selectedServer.gamename}</span></div>
                      <div>Map: <span className="font-bold truncate">{selectedServer.map}</span></div>
                      <div>Players: <span className="font-bold text-green-700">{selectedServer.players}/{selectedServer.maxPlayers}</span></div>
                      <div>Ping: <span className="font-bold">{selectedServer.ping}ms</span></div>
                    </div>
                    <div className="pt-1 flex gap-1">
                      <button
                        onClick={() => handleConnect(selectedServer)}
                        className={`flex-1 py-0.5 text-[10px] font-bold cursor-pointer ${
                          isClassic ? 'ase-win32-btn' : 'bg-slate-800 border text-white'
                        }`}
                      >
                        {copiedId === selectedServer.id ? 'Copied /connect' : 'Copy Connect'}
                      </button>
                      <button
                        onClick={() => handleCopyRawIP(selectedServer)}
                        className={`px-2 py-0.5 text-[10px] cursor-pointer ${
                          isClassic ? 'ase-win32-btn' : 'bg-slate-800 border text-white'
                        }`}
                        title="Copy raw IP"
                      >
                        {copiedRawId === selectedServer.id ? 'IP Copied' : 'IP'}
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="text-slate-500 italic py-4 text-center text-[10px]">
                    No server selected
                  </div>
                )
              ) : (
                <div className="space-y-1 text-[11px]">
                  <div className="text-[10px] text-slate-600 font-semibold mb-1">
                    Favorited Servers ({favorites.size})
                  </div>
                  {favorites.size === 0 ? (
                    <div className="text-slate-400 italic text-[10px]">
                      Click the star icon next to any server to bookmark it here.
                    </div>
                  ) : (
                    Array.from(favorites).slice(0, 8).map((favId) => {
                      const s = data?.servers.find((srv) => srv.id === favId);
                      if (!s) return null;
                      return (
                        <div
                          key={favId}
                          onClick={() => handleSelectServer(s)}
                          className="cursor-pointer hover:bg-[#316ac5] hover:text-white px-1 py-0.5 rounded truncate text-[10px]"
                        >
                          ★ {sanitizeText(s.hostname) || s.ip} ({s.ping}ms)
                        </div>
                      );
                    })
                  )}
                </div>
              )}
            </div>
          </div>

        </div>

        {/* ---------------------------------------------------------------------
            RIGHT PANE: AUTHENTIC SERVER SPREADSHEET & BOTTOM INSPECTOR (9 COLS)
           --------------------------------------------------------------------- */}
        <div className="md:col-span-9 h-full flex flex-col min-h-0 overflow-hidden">
          
          {/* Top Half: Authentic Server Spreadsheet Grid */}
          <div className={`flex-1 min-h-[140px] overflow-auto scrollbar-thin ${
            isClassic ? 'ase-sunken' : 'bg-[#0c1017] border border-slate-800 rounded'
          }`}>
            <table className="w-full text-left text-xs border-collapse">
              <thead className="sticky top-0 select-none z-10">
                <tr>
                  <th className={`py-0.5 px-1.5 w-6 text-center ${isClassic ? 'ase-col-header' : 'bg-slate-800 text-slate-300'}`}>⭐</th>
                  <th className={`py-0.5 px-1.5 w-7 text-center ${isClassic ? 'ase-col-header' : 'bg-slate-800 text-slate-300'}`}>Loc</th>
                  
                  {/* Name Column */}
                  <th 
                    onClick={() => handleSort('name')} 
                    className={`py-0.5 px-2 cursor-pointer ${isClassic ? 'ase-col-header' : 'bg-slate-800 text-slate-300'}`}
                  >
                    <div className="flex items-center gap-1">
                      <span>Name</span>
                      {sortColumn === 'name' && <span>{sortDirection === 'asc' ? '▲' : '▼'}</span>}
                    </div>
                  </th>

                  {/* Ping Column */}
                  <th 
                    onClick={() => handleSort('ping')} 
                    className={`py-0.5 px-2 text-right w-16 cursor-pointer ${isClassic ? 'ase-col-header' : 'bg-slate-800 text-slate-300'}`}
                  >
                    <div className="flex items-center justify-end gap-1">
                      <span>Ping{sortColumn === 'ping' ? '*' : ''}</span>
                      {sortColumn === 'ping' && <span>{sortDirection === 'asc' ? '▲' : '▼'}</span>}
                    </div>
                  </th>

                  {/* Flux Column (Classic ASE ping jitter) */}
                  <th className={`py-0.5 px-1.5 text-center w-12 ${isClassic ? 'ase-col-header' : 'bg-slate-800 text-slate-300'}`}>
                    <span>Flux</span>
                  </th>

                  {/* PL Column (Packet Loss) */}
                  <th className={`py-0.5 px-1.5 text-center w-10 ${isClassic ? 'ase-col-header' : 'bg-slate-800 text-slate-300'}`}>
                    <span>PL</span>
                  </th>

                  {/* Players Column */}
                  <th 
                    onClick={() => handleSort('players')} 
                    className={`py-0.5 px-2 text-center w-20 cursor-pointer ${isClassic ? 'ase-col-header' : 'bg-slate-800 text-slate-300'}`}
                  >
                    <div className="flex items-center justify-center gap-1">
                      <span>Players{sortColumn === 'players' ? '*' : ''}</span>
                      {sortColumn === 'players' && <span>{sortDirection === 'asc' ? '▲' : '▼'}</span>}
                    </div>
                  </th>

                  {/* Game Column */}
                  <th 
                    onClick={() => handleSort('game')} 
                    className={`py-0.5 px-2 w-20 cursor-pointer ${isClassic ? 'ase-col-header' : 'bg-slate-800 text-slate-300'}`}
                  >
                    <div className="flex items-center gap-1">
                      <span>Game</span>
                      {sortColumn === 'game' && <span>{sortDirection === 'asc' ? '▲' : '▼'}</span>}
                    </div>
                  </th>

                  {/* Map Column */}
                  <th 
                    onClick={() => handleSort('map')} 
                    className={`py-0.5 px-2 w-28 cursor-pointer ${isClassic ? 'ase-col-header' : 'bg-slate-800 text-slate-300'}`}
                  >
                    <div className="flex items-center gap-1">
                      <span>Map</span>
                      {sortColumn === 'map' && <span>{sortDirection === 'asc' ? '▲' : '▼'}</span>}
                    </div>
                  </th>

                  {/* Address Column */}
                  <th 
                    onClick={() => handleSort('ip')} 
                    className={`py-0.5 px-2 w-36 cursor-pointer ${isClassic ? 'ase-col-header' : 'bg-slate-800 text-slate-300'}`}
                  >
                    <div className="flex items-center gap-1">
                      <span>Address</span>
                      {sortColumn === 'ip' && <span>{sortDirection === 'asc' ? '▲' : '▼'}</span>}
                    </div>
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 text-[11px] font-sans">
                {isLoading ? (
                  <tr>
                    <td colSpan={10} className="py-12 text-center text-slate-500 font-mono">
                      <div>Querying servers...</div>
                    </td>
                  </tr>
                ) : filteredServers.length === 0 ? (
                  <tr>
                    <td colSpan={10} className="py-12 text-center text-slate-500 font-mono">
                      <div>No servers match filter criteria.</div>
                    </td>
                  </tr>
                ) : (
                  filteredServers.map((srv, index) => {
                    const isSelected = selectedServer?.id === srv.id;
                    const isFav = favorites.has(srv.id);
                    const { flag, country } = getCountryFlag(srv.ip);
                    const cleanName = sanitizeText(srv.hostname) || `${srv.ip}:${srv.gamePort || srv.queryPort}`;
                    const cleanMap = sanitizeText(srv.map) || '---';
                    const maxP = srv.maxPlayers || 32;
                    const isFull = srv.players >= maxP;
                    const flux = `±${(index % 8) + 2}`;

                    return (
                      <tr
                        key={srv.id}
                        onClick={() => handleSelectServer(srv)}
                        onDoubleClick={() => handleConnect(srv)}
                        className={`cursor-pointer h-5 leading-tight ${
                          isSelected
                            ? 'ase-row-selected'
                            : isClassic
                              ? index % 2 === 0 ? 'bg-white hover:bg-[#e8f0ff]' : 'bg-[#f7f6f0] hover:bg-[#e8f0ff]'
                              : 'hover:bg-sky-950/40 odd:bg-[#111620] even:bg-[#0c1017]'
                        }`}
                        title="Click to select, Double-click to copy /connect"
                      >
                        {/* Star */}
                        <td 
                          className="py-0.5 px-1 text-center"
                          onClick={(e) => toggleFavorite(srv.id, e)}
                        >
                          <Star className={`size-3 mx-auto ${isFav ? 'fill-amber-500 text-amber-500' : 'text-slate-300 hover:text-amber-500'}`} />
                        </td>

                        {/* Country Flag */}
                        <td className="py-0.5 px-1 text-center text-xs" title={country}>
                          {flag}
                        </td>

                        {/* Server Name */}
                        <td className="py-0.5 px-2 font-medium truncate max-w-[240px]" title={cleanName}>
                          {cleanName}
                        </td>

                        {/* Ping */}
                        <td className="py-0.5 px-2 text-right font-mono">
                          {srv.ping}
                        </td>

                        {/* Flux */}
                        <td className="py-0.5 px-1.5 text-center text-slate-500 font-mono text-[10px]">
                          {flux}
                        </td>

                        {/* PL */}
                        <td className="py-0.5 px-1.5 text-center text-slate-500 font-mono text-[10px]">
                          0%
                        </td>

                        {/* Players (Green if active, Red if full) */}
                        <td className={`py-0.5 px-2 text-center font-bold ${
                          isFull ? 'text-red-600' : srv.players > 0 ? 'text-green-700' : 'text-slate-500'
                        }`}>
                          {srv.players}/{maxP}
                        </td>

                        {/* Game tag */}
                        <td className="py-0.5 px-2 uppercase text-[10px] truncate">
                          {srv.gamename}
                        </td>

                        {/* Map */}
                        <td className="py-0.5 px-2 truncate max-w-[110px]" title={cleanMap}>
                          {cleanMap}
                        </td>

                        {/* Address: IP:Port */}
                        <td className="py-0.5 px-2 font-mono text-[10px] text-slate-700">
                          {srv.ip}:{srv.gamePort || srv.queryPort}
                        </td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>

          {/* Bottom Half: Authentic Player Table & Server Rules Inspector */}
          <div className="h-[200px] shrink-0 flex flex-col mt-1 overflow-hidden">
            <div className="flex items-center pl-1 z-10">
              <button
                onClick={() => setRightSubTab('players')}
                className={`ase-tab ${rightSubTab === 'players' ? 'ase-tab-active' : ''}`}
              >
                Players ({selectedServer?.roster?.length || selectedServer?.players || 0})
              </button>
              <button
                onClick={() => setRightSubTab('rules')}
                className={`ase-tab ${rightSubTab === 'rules' ? 'ase-tab-active' : ''}`}
              >
                Server Rules &amp; CVars ({selectedServer?.cvars ? Object.keys(selectedServer.cvars).length : 0})
              </button>
            </div>

            <div className={`flex-1 min-h-0 overflow-y-auto scrollbar-thin ${
              isClassic ? 'ase-sunken' : 'bg-[#10141c] border border-slate-800'
            }`}>
              {rightSubTab === 'players' ? (
                selectedServer?.roster && selectedServer.roster.length > 0 ? (
                  <table className="w-full text-left text-xs border-collapse">
                    <thead className="sticky top-0 select-none z-10">
                      <tr>
                        <th className={`py-0.5 px-2 ${isClassic ? 'ase-col-header' : 'bg-slate-800 text-slate-300'}`}>Player Name</th>
                        <th className={`py-0.5 px-2 text-right w-20 ${isClassic ? 'ase-col-header' : 'bg-slate-800 text-slate-300'}`}>Score*</th>
                        <th className={`py-0.5 px-2 text-right w-20 ${isClassic ? 'ase-col-header' : 'bg-slate-800 text-slate-300'}`}>Ping</th>
                        <th className={`py-0.5 px-2 text-right w-20 ${isClassic ? 'ase-col-header' : 'bg-slate-800 text-slate-300'}`}>Time</th>
                        <th className={`py-0.5 px-2 ${isClassic ? 'ase-col-header' : 'bg-slate-800 text-slate-300'}`}>Server</th>
                      </tr>
                    </thead>
                    <tbody className="text-[11px] font-sans">
                      {selectedServer.roster.map((player, idx) => (
                        <tr 
                          key={idx}
                          className={`h-5 ${idx % 2 === 0 ? 'bg-white' : 'bg-[#f7f6f0]'}`}
                        >
                          <td className="py-0.5 px-2 font-medium">
                            🏃 {sanitizeText(player.name) || 'Unnamed Player'}
                          </td>
                          <td className="py-0.5 px-2 text-right font-mono font-bold text-blue-900">
                            {player.frags}
                          </td>
                          <td className="py-0.5 px-2 text-right font-mono text-slate-600">
                            {player.ping > 0 ? `${player.ping}ms` : 'n/a'}
                          </td>
                          <td className="py-0.5 px-2 text-right font-mono text-slate-600">
                            {`${(idx * 7 + 12) % 45}m`}
                          </td>
                          <td className="py-0.5 px-2 text-slate-600 truncate max-w-[200px]">
                            {sanitizeText(selectedServer.hostname)}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                ) : (
                  <div className="py-10 text-center text-slate-500 italic text-xs">
                    {selectedServer ? 'No players currently connected to this server.' : 'Select a server to view player roster.'}
                  </div>
                )
              ) : (
                /* Server Rules & CVars Inspector */
                selectedServer?.cvars && Object.keys(selectedServer.cvars).length > 0 ? (
                  <table className="w-full text-left text-xs border-collapse font-mono">
                    <thead className="sticky top-0 select-none z-10">
                      <tr>
                        <th className={`py-0.5 px-2 w-48 ${isClassic ? 'ase-col-header' : 'bg-slate-800 text-slate-300'}`}>Variable</th>
                        <th className={`py-0.5 px-2 ${isClassic ? 'ase-col-header' : 'bg-slate-800 text-slate-300'}`}>Value</th>
                      </tr>
                    </thead>
                    <tbody className="text-[10px]">
                      {Object.entries(selectedServer.cvars).map(([k, v], idx) => (
                        <tr key={k} className={`h-4.5 ${idx % 2 === 0 ? 'bg-white' : 'bg-[#f7f6f0]'}`}>
                          <td className="py-0.5 px-2 font-bold text-blue-900 border-r border-slate-200">{k}</td>
                          <td className="py-0.5 px-2 truncate max-w-[400px]" title={v}>{v}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                ) : (
                  <div className="py-10 text-center text-slate-500 italic text-xs">
                    {selectedServer ? 'No extended CVars declared for this server.' : 'Select a server to view rules.'}
                  </div>
                )
              )}
            </div>
          </div>

        </div>
      </div>

      {/* =========================================================================
          5. AUTHENTIC WIN32 SUNKEN STATUS BAR (SPANS FULL WINDOW FOOTER)
         ========================================================================= */}
      <div className={`shrink-0 flex items-center gap-1 p-0.5 text-[11px] border-t select-none ${
        isClassic ? 'bg-[#ece9d8] border-[#716f64] text-black' : 'bg-[#19202a] text-slate-300 border-slate-800 font-mono'
      }`}>
        <div className={`flex-1 truncate ${isClassic ? 'ase-statusbar-panel' : 'px-2 py-0.5'}`}>
          <span>{data?.totalServers || 0} servers, {data?.totalPlayers || 0} players total, {filteredServers.length} visible</span>
        </div>
        <div className={`w-44 truncate ${isClassic ? 'ase-statusbar-panel' : 'px-2 py-0.5'}`}>
          <span>Selection: {selectedServer ? sanitizeText(selectedServer.hostname) : 'None'}</span>
        </div>
        <div className={`w-24 text-center ${isClassic ? 'ase-statusbar-panel' : 'px-2 py-0.5'}`}>
          <span>Ping: {selectedServer ? `${selectedServer.ping}ms` : '--'}</span>
        </div>
        <div className={`w-14 text-center font-bold ${isClassic ? 'ase-statusbar-panel' : 'px-2 py-0.5 text-sky-400'}`}>
          <span>NUM</span>
        </div>
        <div className={`w-16 text-center font-bold ${isClassic ? 'ase-statusbar-panel' : 'px-2 py-0.5 text-emerald-400'}`}>
          <span>READY</span>
        </div>
      </div>

      {/* =========================================================================
          CATALOG DRAWER (4,678 GAMES DIRECTORY)
         ========================================================================= */}
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

    </div>
  );
}
