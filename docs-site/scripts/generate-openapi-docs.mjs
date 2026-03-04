import { generateFiles } from 'fumadocs-openapi';
import { createOpenAPI } from 'fumadocs-openapi/server';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const openapi = createOpenAPI({
  input: [path.resolve(__dirname, '../openapi.json')],
});

await generateFiles({
  input: openapi,
  output: path.resolve(__dirname, '../content/docs/api-reference'),
  groupBy: 'tag',
  includeDescription: true,
});
