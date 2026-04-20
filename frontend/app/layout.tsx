import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "MakeSense.ai",
  description: "Write anything. See it organized instantly.",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
