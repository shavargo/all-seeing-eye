import type { Metadata } from 'next';
import { Inter } from 'next/font/google';
import './globals.css';

const inter = Inter({ subsets: ['latin'] });

export const metadata: Metadata = {
  title: 'The All-Seeing Eye | Universal Master Server Browser',
  description: 'Modernized revival of The All-Seeing Eye launcher for 4,841+ Quake, idTech, and GameSpy multiplayer games.',
  icons: {
    icon: '/ase-logo.png',
    apple: '/ase-logo.png',
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="h-full overflow-hidden">
      <body className={`${inter.className} h-full h-screen max-h-screen overflow-hidden bg-[#0d1117] text-slate-100 antialiased`}>
        {children}
      </body>
    </html>
  );
}
