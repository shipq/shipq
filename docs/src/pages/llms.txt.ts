import type { APIRoute } from 'astro';
import fs from 'node:fs';
import path from 'node:path';

export const prerender = true;

export const GET: APIRoute = async () => {
  const filePath = path.resolve('src/content/docs/llms.md');
  const raw = fs.readFileSync(filePath, 'utf-8');

  // Strip YAML frontmatter (everything between the opening and closing ---)
  const stripped = raw.replace(/^---[\s\S]*?---\n*/, '');

  return new Response(stripped.trim() + '\n', {
    headers: {
      'Content-Type': 'text/plain; charset=utf-8',
    },
  });
};
