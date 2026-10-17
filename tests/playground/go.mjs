// [INPUT]: Repository MyGo patches and centrally stored Go tests.
// [OUTPUT]: Native go test build arguments; standalone validation scripts and Make share the same preparation entry.
// [POS]: Test-only command preparation, preserving Go arguments, subprocess exit codes, and platform selection.
// [PROTOCOL]: Update tests/AGENTS.md and both platform validation scripts.
import { spawnSync } from 'node:child_process';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

export const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');

export function goTestFlags() {
  return [['modfile', './tools/mygo-patch'], ['overlay', './tools/test-overlay']].map(([flag, tool]) => {
    const result = spawnSync('go', ['run', tool], { cwd: root, encoding: 'utf8' });
    if (result.error) throw result.error;
    if (result.status !== 0 || !result.stdout.trim()) throw new Error(`${tool}: ${result.stderr || '未生成构建配置'}`);
    return `-${flag}=${result.stdout.trim()}`;
  });
}
