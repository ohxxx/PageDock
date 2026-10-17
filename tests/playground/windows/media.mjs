// [INPUT]: A Windows Go test environment, or a compiled desktop test executable supplied as the first argument.
// [OUTPUT]: Audio/video tests using temporary WAV/Y4M devices and isolated WebView2 profiles.
// [POS]: Windows validation entry; replace device input without auto-granting permission; update tests/AGENTS.md and playground/AGENTS.md.
import { mkdtemp, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { spawn } from 'node:child_process';
import { root, goTestFlags } from '../go.mjs';

if (process.platform !== 'win32') throw new Error('此命令需要 Windows 桌面与 WebView2 Runtime');
const dir = await mkdtemp(join(tmpdir(), 'pagedock-media-'));
try {
  const rate = 48000, seconds = 4, samples = rate * seconds;
  const audio = Buffer.alloc(44 + samples * 2);
  audio.write('RIFF'); audio.writeUInt32LE(audio.length - 8, 4); audio.write('WAVEfmt ', 8);
  audio.writeUInt32LE(16, 16); audio.writeUInt16LE(1, 20); audio.writeUInt16LE(1, 22);
  audio.writeUInt32LE(rate, 24); audio.writeUInt32LE(rate * 2, 28);
  audio.writeUInt16LE(2, 32); audio.writeUInt16LE(16, 34); audio.write('data', 36);
  audio.writeUInt32LE(samples * 2, 40);
  for (let i = 0; i < samples; i++) audio.writeInt16LE(Math.round(8000 * Math.sin(2 * Math.PI * 440 * i / rate)), 44 + i * 2);
  const width = 320, height = 240, fps = 20, pixels = width * height;
  const frames = [Buffer.from(`YUV4MPEG2 W${width} H${height} F${fps}:1 Ip A1:1 C420jpeg\n`)];
  for (let i = 0; i < fps * seconds; i++) frames.push(
    Buffer.from('FRAME\n'), Buffer.alloc(pixels, 32 + i % fps * 10),
    Buffer.alloc(pixels / 4, 80 + i % fps * 5), Buffer.alloc(pixels / 4, 180 - i % fps * 5),
  );
  const wav = join(dir, 'input.wav'), y4m = join(dir, 'input.y4m');
  await Promise.all([writeFile(wav, audio), writeFile(y4m, Buffer.concat(frames))]);
  // Built-in test devices may yield silence/no frames on this ARM64 Runtime; validate with deterministic file sources.
  const env = { ...process.env, LOCALAPPDATA: dir, WEBVIEW2_USER_DATA_FOLDER: '', WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS:
    `--use-fake-device-for-media-stream --use-file-for-fake-audio-capture="${wav}" --use-file-for-fake-video-capture="${y4m}"` };
  const executable = process.argv[2];
  const command = executable ? resolve(executable) : 'go';
  const args = executable ? ['-test.run=^TestDesktopWindowsMediaPermissionRecovery$', '-test.v', '-test.timeout=90s']
    : ['test', ...goTestFlags(), '-tags', 'desktop', '-run', '^TestDesktopWindowsMediaPermissionRecovery$', '-count=1', '-v', '-timeout=90s', './internal'];
  const child = spawn(command, args, { cwd: root, env, stdio: 'inherit' });
  process.exitCode = await new Promise((resolve, reject) => {
    child.once('error', reject);
    child.once('exit', (code, signal) => resolve(signal ? 1 : (code ?? 1)));
  });
} finally {
  await rm(dir, { recursive: true, force: true, maxRetries: 30, retryDelay: 100 });
}
