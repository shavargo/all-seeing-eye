'use client';

import React from 'react';
import Image from 'next/image';
import { 
  Users, 
  Search, 
  Play, 
  Zap, 
  X, 
  Moon,
  Folder,
  FolderOpen,
  Volume2,
  VolumeX,
  Star,
  Gamepad2,
  Globe
} from 'lucide-react';

export interface ClassicThemeViewProps {
  data: any;
  isLoading: boolean;
  mutate: () => void;
  setTheme: (theme: 'dark' | 'classic') => void;
  audioEnabled: boolean;
  toggleAudio: () => void;
  favorites: Set<string>;
  toggleFavorite: (id: string, e?: React.MouseEvent) => void;
  history: string[];
  selectedServer: any | null;
  handleSelectServer: (srv: any) => void;
  handleConnect: (srv: any) => void;
  handleCopyRawIP: (srv: any) => void;
  treeSelection: string;
  setTreeSelection: (s: string) => void;
  regionFilter: string;
  setRegionFilter: (r: string) => void;
  searchQuery: string;
  setSearchQuery: (q: string) => void;
  hideEmpty: boolean;
  setHideEmpty: (b: boolean) => void;
  hideFull: boolean;
  setHideFull: (b: boolean) => void;
  serverTypeFilter: string;
  setServerTypeFilter: (t: string) => void;
  sortColumn: string;
  sortDirection: 'asc' | 'desc';
  handleSort: (col: any) => void;
  showGamesFolder: boolean;
  setShowGamesFolder: React.Dispatch<React.SetStateAction<boolean>>;
  showRegionsFolder: boolean;
  setShowRegionsFolder: React.Dispatch<React.SetStateAction<boolean>>;
  showServerTypeFolder: boolean;
  setShowServerTypeFolder: React.Dispatch<React.SetStateAction<boolean>>;
  leftSubTab: 'buddy' | 'server';
  setLeftSubTab: (t: 'buddy' | 'server') => void;
  rightSubTab: 'players' | 'rules';
  setRightSubTab: (t: 'players' | 'rules') => void;
  filteredServers: any[];
  activeGameDistribution: any[];
  getCountryFlag: (ip: string) => { flag: string; country: string };
  sanitizeText: (str?: string) => string;
  copiedId: string | null;
  copiedRawId: string | null;
  setCatalogOpen: (b: boolean) => void;
}

export const ClassicThemeView: React.FC<ClassicThemeViewProps> = ({
  data,
  isLoading,
  mutate,
  setTheme,
  audioEnabled,
  toggleAudio,
  favorites,
  toggleFavorite,
  history,
  selectedServer,
  handleSelectServer,
  handleConnect,
  handleCopyRawIP,
  treeSelection,
  setTreeSelection,
  regionFilter,
  setRegionFilter,
  searchQuery,
  setSearchQuery,
  hideEmpty,
  setHideEmpty,
  hideFull,
  setHideFull,
  serverTypeFilter,
  setServerTypeFilter,
  sortColumn,
  sortDirection,
  handleSort,
  showGamesFolder,
  setShowGamesFolder,
  showRegionsFolder,
  setShowRegionsFolder,
  showServerTypeFolder,
  setShowServerTypeFolder,
  leftSubTab,
  setLeftSubTab,
  rightSubTab,
  setRightSubTab,
  filteredServers,
  activeGameDistribution,
  getCountryFlag,
  sanitizeText,
  copiedId,
  copiedRawId,
  setCatalogOpen,
}) => {
  return (
    <div className="h-screen max-h-screen h-dvh max-h-dvh flex flex-col overflow-hidden select-none ase-classic">

      {/* =========================================================================
          1. CLASSIC WINDOWS XP / 2000 TITLEBAR
         ========================================================================= */}
      <div className="shrink-0 flex items-center justify-between px-2 py-0.5 border-b select-none ase-titlebar border-[#716f64]">
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
          <span>The All-Seeing Eye - [{data?.totalServers?.toLocaleString() || '0'} servers, {data?.totalPlayers?.toLocaleString() || '0'} players]</span>
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
      <div className="shrink-0 px-2 py-0.5 flex items-center gap-3 text-[11px] border-b bg-[#ece9d8] border-[#716f64] text-black">
        {['File', 'View', 'Filter', 'Server', 'Player', 'Buddy', 'Channels', 'Tools', 'Community', 'Help'].map((menu) => (
          <span 
            key={menu} 
            className="px-1.5 py-0.2 cursor-pointer transition-colors hover:bg-[#0a246a] hover:text-white"
          >
            {menu}
          </span>
        ))}
      </div>

      {/* =========================================================================
          3. AUTHENTIC 3D RAISED TOOLBAR
         ========================================================================= */}
      <div className="shrink-0 p-1 flex items-center flex-wrap gap-1.5 border-b text-[11px] ase-raised border-b-[#716f64]">
        {/* Refresh Master Button */}
        <button 
          onClick={() => mutate()}
          className="ase-win32-btn px-2 py-0.5 flex items-center gap-1.5 cursor-pointer"
          title="Refresh servers from master (F5)"
        >
          <span className="text-xs">👁️</span>
          <span className="font-semibold">Refresh</span>
        </button>

        {/* Stop Button */}
        <button 
          type="button"
          className="ase-win32-btn px-2 py-0.5 flex items-center gap-1 cursor-pointer"
          title="Stop scanning"
        >
          <span className="text-red-600 font-bold">■</span>
          <span>Stop</span>
        </button>

        {/* Ping Visible Button */}
        <button 
          onClick={() => mutate()}
          className="ase-win32-btn px-2 py-0.5 flex items-center gap-1 cursor-pointer"
          title="Ping visible servers"
        >
          <Zap className="size-3 text-amber-500 fill-amber-500" />
          <span>Ping</span>
        </button>

        {/* Quick Filter Search Input */}
        <div className="flex items-center gap-1 ml-1">
          <span className="text-slate-600 font-semibold text-[10px]">Filter:</span>
          <div className="ase-sunken flex items-center px-1.5 py-0.5">
            <Search className="size-3 text-slate-500 mr-1" />
            <input 
              id="ase-filter-search"
              type="text" 
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search..." 
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
          onClick={() => setHideEmpty(!hideEmpty)}
          className={`px-2 py-0.5 flex items-center gap-1 cursor-pointer ${
            hideEmpty ? 'ase-win32-btn-active font-bold text-blue-900' : 'ase-win32-btn text-slate-700'
          }`}
          title="Hide empty servers"
        >
          <span>[ Not Empty ]</span>
        </button>

        {/* Not Full Filter Toggle */}
        <button
          onClick={() => setHideFull(!hideFull)}
          className={`px-2 py-0.5 flex items-center gap-1 cursor-pointer ${
            hideFull ? 'ase-win32-btn-active font-bold text-blue-900' : 'ase-win32-btn text-slate-700'
          }`}
          title="Hide full servers"
        >
          <span>[ Not Full ]</span>
        </button>

        {/* Connect to Game Button */}
        {selectedServer && (
          <button 
            onClick={() => handleConnect(selectedServer)}
            className="ase-win32-btn px-2.5 py-0.5 font-bold cursor-pointer transition-colors flex items-center gap-1 text-emerald-800"
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
            className="ase-win32-btn px-2 py-0.5 flex items-center gap-1 cursor-pointer"
            title="Toggle synthesized SFX"
          >
            {audioEnabled ? <Volume2 className="size-3 text-emerald-600" /> : <VolumeX className="size-3 text-slate-400" />}
            <span>SFX</span>
          </button>

          {/* Switch back to Modern Dark Theme */}
          <button
            onClick={() => setTheme('dark')}
            className="ase-win32-btn px-2 py-0.5 flex items-center gap-1 cursor-pointer font-bold text-blue-900"
            title="Switch back to Modern Dark Theme"
          >
            <Moon className="size-3 text-blue-700" />
            <span>Dark Theme</span>
          </button>

          {/* Master Catalog Drawer */}
          <button
            onClick={() => setCatalogOpen(true)}
            className="ase-win32-btn px-2 py-0.5 flex items-center gap-1 cursor-pointer"
            title="Open Master Games Database"
          >
            <span>Games Catalog ▼</span>
          </button>
        </div>
      </div>

      {/* =========================================================================
          4. MAIN 2-COLUMN SPLIT: LEFT DIRECTORY TREE | RIGHT SERVER LIST & TABS
         ========================================================================= */}
      <div className="flex-1 min-h-0 flex flex-row overflow-hidden p-1 gap-1">
        
        {/* ---------------------------------------------------------------------
            LEFT PANE: AUTHENTIC WIN32 TREEVIEW (Sunken 3D Box)
           --------------------------------------------------------------------- */}
        <div className="w-56 shrink-0 h-full ase-sunken overflow-y-auto p-1 text-[11px] flex flex-col">
          
          {/* Root Node: The All-Seeing Eye */}
          <div className="font-bold flex items-center gap-1 py-0.5 px-1 bg-[#ece9d8] border-b border-[#aca899] mb-1">
            <span className="text-[10px]">👁️</span>
            <span>The All-Seeing Eye</span>
          </div>

          {/* Category: Games */}
          <div className="mb-1">
            <div 
              onClick={() => setShowGamesFolder(!showGamesFolder)}
              className="flex items-center gap-1 py-0.5 px-1 hover:bg-[#c1d2ee] cursor-pointer"
            >
              <span className="ase-tree-expander">{showGamesFolder ? '-' : '+'}</span>
              <FolderOpen className="size-3 text-amber-600 fill-amber-200" />
              <span className="font-semibold">Games ({activeGameDistribution.length})</span>
            </div>

            {showGamesFolder && (
              <div className="pl-4 space-y-0.5 border-l border-dotted border-slate-400 ml-2 mt-0.5">
                {/* All Servers Node */}
                <div 
                  onClick={() => {
                    setTreeSelection('all');
                    setRegionFilter('all');
                  }}
                  className={`flex items-center justify-between py-0.5 px-1.5 cursor-pointer ${
                    treeSelection === 'all' && regionFilter === 'all' ? 'ase-row-selected' : 'hover:bg-[#c1d2ee]'
                  }`}
                >
                  <div className="flex items-center gap-1.5 truncate">
                    <span className="text-[9px]">📄</span>
                    <span>All Active</span>
                  </div>
                  <span className="text-[9px] font-bold px-1">{data?.totalServers || 0}</span>
                </div>

                {/* Per-Game Subnodes */}
                {activeGameDistribution.map((game) => {
                  const isSelected = treeSelection.toLowerCase() === game.gameKey.toLowerCase();
                  return (
                    <div
                      key={game.gameKey}
                      onClick={() => {
                        setTreeSelection(game.gameKey);
                        setRegionFilter('all');
                      }}
                      className={`flex items-center justify-between py-0.5 px-1.5 cursor-pointer ${
                        isSelected ? 'ase-row-selected' : 'hover:bg-[#c1d2ee]'
                      }`}
                    >
                      <div className="flex items-center gap-1.5 truncate">
                        {game.icon ? (
                          <div className="relative size-3 rounded shrink-0">
                            <Image src={game.icon} alt={game.displayName} fill sizes="12px" className="object-cover" />
                          </div>
                        ) : (
                          <Gamepad2 className="size-3 text-slate-600 shrink-0" />
                        )}
                        <span className="truncate">{game.displayName}</span>
                      </div>
                      <span className="text-[9px] font-bold px-1">{game.serverCount}</span>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          {/* Category: Territories / Countries */}
          <div className="mb-1">
            <div 
              onClick={() => setShowRegionsFolder(!showRegionsFolder)}
              className="flex items-center gap-1 py-0.5 px-1 hover:bg-[#c1d2ee] cursor-pointer"
            >
              <span className="ase-tree-expander">{showRegionsFolder ? '-' : '+'}</span>
              <Folder className="size-3 text-amber-600 fill-amber-200" />
              <span className="font-semibold">Territories</span>
            </div>

            {showRegionsFolder && (
              <div className="pl-4 space-y-0.5 border-l border-dotted border-slate-400 ml-2 mt-0.5">
                {[
                  { name: 'North America', flag: '🇺🇸' },
                  { name: 'Europe', flag: '🇪🇺' },
                  { name: 'Germany', flag: '🇩🇪' },
                  { name: 'United Kingdom', flag: '🇬🇧' },
                  { name: 'Asia-Pacific', flag: '🌏' }
                ].map((reg) => (
                  <div
                    key={reg.name}
                    onClick={() => setRegionFilter(regionFilter === reg.name ? 'all' : reg.name)}
                    className={`flex items-center justify-between py-0.5 px-1.5 cursor-pointer ${
                      regionFilter === reg.name ? 'ase-row-selected' : 'hover:bg-[#c1d2ee]'
                    }`}
                  >
                    <div className="flex items-center gap-1 truncate">
                      <span>{reg.flag}</span>
                      <span>{reg.name}</span>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Category: Server Types */}
          <div className="mb-1">
            <div 
              onClick={() => setShowServerTypeFolder(!showServerTypeFolder)}
              className="flex items-center gap-1 py-0.5 px-1 hover:bg-[#c1d2ee] cursor-pointer"
            >
              <span className="ase-tree-expander">{showServerTypeFolder ? '-' : '+'}</span>
              <Folder className="size-3 text-amber-600 fill-amber-200" />
              <span className="font-semibold">Server Types</span>
            </div>

            {showServerTypeFolder && (
              <div className="pl-4 space-y-0.5 border-l border-dotted border-slate-400 ml-2 mt-0.5">
                {[
                  { label: 'All Types', val: 'all' },
                  { label: 'Pure / Standard', val: 'pure' },
                  { label: 'Dedicated Only', val: 'dedicated' },
                ].map((t) => (
                  <div
                    key={t.val}
                    onClick={() => setServerTypeFilter(t.val)}
                    className={`flex items-center justify-between py-0.5 px-1.5 cursor-pointer ${
                      serverTypeFilter === t.val ? 'ase-row-selected' : 'hover:bg-[#c1d2ee]'
                    }`}
                  >
                    <span>{t.label}</span>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Quick Access: Favorites & History */}
          <div className="mt-auto pt-1 border-t border-[#aca899] space-y-0.5">
            <div 
              onClick={() => setTreeSelection('favorites')}
              className={`flex items-center justify-between py-0.5 px-1.5 cursor-pointer ${
                treeSelection === 'favorites' ? 'ase-row-selected' : 'hover:bg-[#c1d2ee]'
              }`}
            >
              <div className="flex items-center gap-1">
                <Star className="size-3 text-amber-500 fill-amber-500" />
                <span>Favorites</span>
              </div>
              <span className="text-[9px] font-bold">{favorites.size}</span>
            </div>

            <div 
              onClick={() => setTreeSelection('history')}
              className={`flex items-center justify-between py-0.5 px-1.5 cursor-pointer ${
                treeSelection === 'history' ? 'ase-row-selected' : 'hover:bg-[#c1d2ee]'
              }`}
            >
              <div className="flex items-center gap-1">
                <span className="text-[10px]">🕒</span>
                <span>History</span>
              </div>
              <span className="text-[9px] font-bold">{history.length}</span>
            </div>
          </div>
        </div>

        {/* ---------------------------------------------------------------------
            RIGHT PANE: TOP SERVER TABLE (Sunken) + BOTTOM INSPECTOR TABS (Sunken)
           --------------------------------------------------------------------- */}
        <div className="flex-1 min-w-0 h-full flex flex-col gap-1 overflow-hidden">
          
          {/* TOP: WIN32 SERVER DATA GRID (Takes upper 60% of vertical space) */}
          <div className="flex-1 min-h-0 ase-sunken overflow-auto bg-white">
            <table className="w-full border-collapse text-[11px] text-left select-none">
              <thead className="sticky top-0 z-10">
                <tr className="bg-[#ece9d8]">
                  <th className="ase-col-header py-1 px-1.5 w-6 text-center">⭐</th>
                  <th className="ase-col-header py-1 px-1.5 w-7 text-center">Loc</th>
                  <th onClick={() => handleSort('game')} className="ase-col-header py-1 px-2 w-28 cursor-pointer">
                    Game {sortColumn === 'game' && (sortDirection === 'asc' ? '▲' : '▼')}
                  </th>
                  <th onClick={() => handleSort('name')} className="ase-col-header py-1 px-2 cursor-pointer">
                    Server Name {sortColumn === 'name' && (sortDirection === 'asc' ? '▲' : '▼')}
                  </th>
                  <th onClick={() => handleSort('ping')} className="ase-col-header py-1 px-2 text-right w-16 cursor-pointer">
                    Ping {sortColumn === 'ping' && (sortDirection === 'asc' ? '▲' : '▼')}
                  </th>
                  <th onClick={() => handleSort('players')} className="ase-col-header py-1 px-2 text-center w-20 cursor-pointer">
                    Players {sortColumn === 'players' && (sortDirection === 'asc' ? '▲' : '▼')}
                  </th>
                  <th onClick={() => handleSort('map')} className="ase-col-header py-1 px-2 w-32 cursor-pointer">
                    Map {sortColumn === 'map' && (sortDirection === 'asc' ? '▲' : '▼')}
                  </th>
                  <th onClick={() => handleSort('type')} className="ase-col-header py-1 px-2 w-24 cursor-pointer">
                    Type {sortColumn === 'type' && (sortDirection === 'asc' ? '▲' : '▼')}
                  </th>
                  <th onClick={() => handleSort('ip')} className="ase-col-header py-1 px-2 w-36 cursor-pointer">
                    IP Address {sortColumn === 'ip' && (sortDirection === 'asc' ? '▲' : '▼')}
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[#e0ded6] bg-white">
                {isLoading ? (
                  <tr>
                    <td colSpan={9} className="py-8 text-center text-slate-500 italic">
                      Querying master server...
                    </td>
                  </tr>
                ) : filteredServers.length === 0 ? (
                  <tr>
                    <td colSpan={9} className="py-8 text-center text-slate-500 italic">
                      No servers found matching current filters.
                    </td>
                  </tr>
                ) : (
                  filteredServers.map((srv) => {
                    const isSelected = selectedServer?.id === srv.id;
                    const isFav = favorites.has(srv.id);
                    const { flag } = getCountryFlag(srv.ip);
                    const cleanName = sanitizeText(srv.hostname) || `${srv.ip}:${srv.gamePort || srv.queryPort}`;
                    const cleanMap = sanitizeText(srv.map) || '---';
                    const maxP = srv.maxPlayers || 32;

                    return (
                      <tr
                        key={srv.id}
                        onClick={() => handleSelectServer(srv)}
                        onDoubleClick={() => handleConnect(srv)}
                        className={`cursor-pointer leading-tight ${
                          isSelected ? 'ase-row-selected' : 'hover:bg-[#eef4fa] odd:bg-white even:bg-[#fbfaf6]'
                        }`}
                      >
                        <td 
                          className="py-0.5 px-1 text-center" 
                          onClick={(e) => toggleFavorite(srv.id, e)}
                        >
                          <Star className={`size-3 mx-auto ${isFav ? 'fill-amber-500 text-amber-500' : 'text-slate-300'}`} />
                        </td>
                        <td className="py-0.5 px-1 text-center text-xs">{flag}</td>
                        <td className="py-0.5 px-2 font-semibold truncate uppercase text-[10px]">{srv.gamename}</td>
                        <td className="py-0.5 px-2 font-medium truncate max-w-[280px]" title={cleanName}>{cleanName}</td>
                        <td className={`py-0.5 px-2 text-right ${isSelected ? 'text-white' : srv.ping <= 60 ? 'text-emerald-700 font-bold' : srv.ping <= 120 ? 'text-amber-700' : 'text-rose-700'}`}>
                          {srv.ping}ms
                        </td>
                        <td className="py-0.5 px-2 text-center font-bold">{srv.players}/{maxP}</td>
                        <td className="py-0.5 px-2 truncate max-w-[120px]" title={cleanMap}>{cleanMap}</td>
                        <td className="py-0.5 px-2 truncate text-slate-600">{srv.gametype || 'Normal'}</td>
                        <td className="py-0.5 px-2 font-mono text-[10px] text-slate-600">{srv.ip}:{srv.gamePort || srv.queryPort}</td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>

          {/* BOTTOM: THE 2-PANE INSPECTOR WITH AUTHENTIC TABS (Height 190px) */}
          <div className="h-48 shrink-0 grid grid-cols-1 md:grid-cols-2 gap-1 overflow-hidden">
            
            {/* BOTTOM LEFT: SERVER / BUDDY TAB */}
            <div className="flex flex-col h-full overflow-hidden">
              <div className="flex items-center px-1 pt-1 bg-[#ece9d8] border-b border-[#716f64]">
                <button
                  onClick={() => setLeftSubTab('server')}
                  className={`ase-tab ${leftSubTab === 'server' ? 'ase-tab-active' : ''}`}
                >
                  Server Info
                </button>
                <button
                  onClick={() => setLeftSubTab('buddy')}
                  className={`ase-tab ${leftSubTab === 'buddy' ? 'ase-tab-active' : ''}`}
                >
                  Buddy List
                </button>
              </div>

              <div className="flex-1 min-h-0 ase-sunken p-2 overflow-y-auto bg-white text-[11px]">
                {leftSubTab === 'server' ? (
                  selectedServer ? (
                    <div className="space-y-1">
                      <div className="font-bold text-[#0a246a] text-xs pb-1 border-b border-[#e0ded6] truncate">
                        {sanitizeText(selectedServer.hostname)}
                      </div>
                      <div className="grid grid-cols-2 gap-x-2 gap-y-0.5 pt-1 text-[10px]">
                        <div><span className="text-slate-500">Address: </span><code className="bg-slate-100 px-1 font-bold">{selectedServer.ip}:{selectedServer.gamePort || selectedServer.queryPort}</code></div>
                        <div><span className="text-slate-500">Ping: </span><span className="font-bold text-emerald-700">{selectedServer.ping}ms</span></div>
                        <div><span className="text-slate-500">Game: </span><span className="font-semibold uppercase">{selectedServer.gamename}</span></div>
                        <div><span className="text-slate-500">Map: </span><span className="font-semibold">{selectedServer.map || '---'}</span></div>
                        <div><span className="text-slate-500">Gametype: </span><span>{selectedServer.gametype || 'Standard'}</span></div>
                        <div><span className="text-slate-500">Players: </span><span className="font-bold text-amber-700">{selectedServer.players}/{selectedServer.maxPlayers || 32}</span></div>
                      </div>

                      <div className="pt-2 flex items-center gap-1.5 border-t border-[#e0ded6] mt-2">
                        <button
                          onClick={() => handleCopyRawIP(selectedServer)}
                          className="ase-win32-btn px-2 py-0.5 text-[10px]"
                          title="Copy IP:Port to clipboard"
                        >
                          {copiedRawId === selectedServer.id ? 'IP COPIED!' : 'Copy IP'}
                        </button>
                        <button
                          onClick={() => handleConnect(selectedServer)}
                          className="ase-win32-btn px-2.5 py-0.5 text-[10px] font-bold text-emerald-800"
                          title="Copy /connect console command"
                        >
                          {copiedId === selectedServer.id ? 'COPIED!' : 'Copy /connect'}
                        </button>
                      </div>
                    </div>
                  ) : (
                    <div className="h-full flex items-center justify-center text-slate-400 italic">
                      Select a server from the list above.
                    </div>
                  )
                ) : (
                  <div className="h-full flex flex-col items-center justify-center text-slate-500 p-2 text-center">
                    <p className="font-semibold">No buddies online</p>
                    <p className="text-[10px] text-slate-400">Add player names to monitor their presence across all servers.</p>
                  </div>
                )}
              </div>
            </div>

            {/* BOTTOM RIGHT: PLAYER ROSTER / RULES TAB */}
            <div className="flex flex-col h-full overflow-hidden">
              <div className="flex items-center px-1 pt-1 bg-[#ece9d8] border-b border-[#716f64]">
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
                  Server Rules ({selectedServer?.cvars ? Object.keys(selectedServer.cvars).length : 0})
                </button>
              </div>

              <div className="flex-1 min-h-0 ase-sunken overflow-y-auto bg-white text-[11px]">
                {rightSubTab === 'players' ? (
                  selectedServer ? (
                    selectedServer.roster && selectedServer.roster.length > 0 ? (
                      <table className="w-full text-left border-collapse text-[11px]">
                        <thead>
                          <tr className="bg-[#ece9d8] text-black border-b border-[#aca899]">
                            <th className="py-0.5 px-2 font-normal">Player Name</th>
                            <th className="py-0.5 px-2 font-normal text-right w-16">Frags</th>
                            <th className="py-0.5 px-2 font-normal text-right w-16">Ping</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-[#e0ded6]">
                          {selectedServer.roster.map((p: any, idx: number) => (
                            <tr key={idx} className="hover:bg-[#eef4fa]">
                              <td className="py-0.5 px-2 font-medium">{sanitizeText(p.name)}</td>
                              <td className="py-0.5 px-2 text-right font-mono text-amber-700">{p.frags}</td>
                              <td className="py-0.5 px-2 text-right font-mono text-emerald-700">{p.ping}ms</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    ) : selectedServer.players > 0 ? (
                      <div className="p-3 text-center text-slate-500 italic text-[11px]">
                        {selectedServer.players} players active on server (combat roster streaming).
                      </div>
                    ) : (
                      <div className="h-full flex items-center justify-center text-slate-400 italic">
                        No players currently connected.
                      </div>
                    )
                  ) : (
                    <div className="h-full flex items-center justify-center text-slate-400 italic">
                      Select a server to view player roster.
                    </div>
                  )
                ) : (
                  selectedServer?.cvars && Object.keys(selectedServer.cvars).length > 0 ? (
                    <table className="w-full text-left border-collapse text-[10px] font-mono">
                      <thead>
                        <tr className="bg-[#ece9d8] text-black border-b border-[#aca899]">
                          <th className="py-0.5 px-2 font-normal w-1/3">Rule / Variable</th>
                          <th className="py-0.5 px-2 font-normal">Value</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-[#e0ded6]">
                        {Object.entries(selectedServer.cvars).map(([k, v]) => (
                          <tr key={k} className="hover:bg-[#eef4fa]">
                            <td className="py-0.5 px-2 font-bold text-slate-700">{k}</td>
                            <td className="py-0.5 px-2 text-slate-900 truncate max-w-[200px]" title={String(v)}>{String(v)}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  ) : (
                    <div className="h-full flex items-center justify-center text-slate-400 italic">
                      No extended server CVars declared.
                    </div>
                  )
                )}
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* =========================================================================
          5. AUTHENTIC WIN32 STATUS BAR (3 Sunken Boxes)
         ========================================================================= */}
      <div className="shrink-0 p-0.5 flex items-center gap-1 bg-[#ece9d8] border-t border-[#716f64] text-[11px]">
        {/* Status Box 1: Server and Player Counts */}
        <div className="ase-statusbar-panel flex-1 truncate">
          <span>{filteredServers.length} of {data?.totalServers || 0} servers shown, {data?.totalPlayers || 0} players online</span>
        </div>

        {/* Status Box 2: Selected Server Quick Info */}
        <div className="ase-statusbar-panel w-64 truncate">
          {selectedServer ? (
            <span>{selectedServer.gamename.toUpperCase()} - {selectedServer.ip}:{selectedServer.gamePort || selectedServer.queryPort}</span>
          ) : (
            <span>Ready</span>
          )}
        </div>

        {/* Status Box 3: Network / Refresh Status */}
        <div className="ase-statusbar-panel w-32 flex items-center justify-between">
          <span className="flex items-center gap-1">
            <span className="size-2 rounded-full bg-emerald-500 inline-block animate-pulse"></span>
            <span>Online</span>
          </span>
          <span className="text-[10px] text-slate-500">10.0s</span>
        </div>
      </div>

    </div>
  );
};
