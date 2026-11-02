import { NextResponse } from 'next/server';

const MASTER_API_URL = process.env.MASTER_API_URL || 'http://127.0.0.1:8085';

export async function GET(request: Request) {
  const { searchParams } = new URL(request.url);
  const q = searchParams.get('q') || '';
  const category = searchParams.get('category') || '';
  const limit = searchParams.get('limit') || '50';

  try {
    const params = new URLSearchParams();
    if (q) params.set('q', q);
    if (category) params.set('category', category);
    if (limit) params.set('limit', limit);

    const res = await fetch(`${MASTER_API_URL}/api/v1/games?${params.toString()}`, {
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
        'Cache-Control': 'public, s-maxage=30, stale-while-revalidate=60',
      },
    });
  } catch (error) {
    return NextResponse.json({
      total: 4841,
      matchCount: 0,
      games: [],
      error: 'Game catalog currently offline or initializing',
    });
  }
}
