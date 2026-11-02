import { NextResponse } from 'next/server';

const MASTER_API_URL = process.env.MASTER_API_URL || 'http://127.0.0.1:8085';

export async function GET(request: Request) {
  const { searchParams } = new URL(request.url);
  const game = searchParams.get('game') || 'all';

  try {
    const res = await fetch(`${MASTER_API_URL}/api/v1/servers?game=${encodeURIComponent(game)}`, {
      cache: 'no-store',
      headers: {
        Accept: 'application/json',
      },
    });

    if (!res.ok) {
      throw new Error(`Master daemon responded with HTTP ${res.status}`);
    }

    const data = await res.json();
    return NextResponse.json(data, {
      headers: {
        'Cache-Control': 'public, s-maxage=5, stale-while-revalidate=10',
      },
    });
  } catch (error) {
    // Return empty safe snapshot instead of a 500 error to keep UI resilient
    return NextResponse.json({
      generatedAt: new Date().toISOString(),
      totalServers: 0,
      totalPlayers: 0,
      activeGames: 0,
      servers: [],
      error: 'Master server snapshot currently offline or initializing',
    });
  }
}
