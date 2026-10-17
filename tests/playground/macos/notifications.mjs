// [INPUT]: A macOS desktop, existing MyGo Dev bundle, Go/Node toolchains, and notification authorization.
// [OUTPUT]: Real notification tests retaining the current icon, with per-message presentation evidence; unregister the test bundle afterward.
// [POS]: Manual validation entry outside application builds; preserve system notification preferences.
// [PROTOCOL]: Update tests/AGENTS.md, playground/AGENTS.md, and validation records.
import { copyFileSync, cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, renameSync } from 'node:fs';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import { root, goTestFlags } from '../go.mjs';

if (process.platform !== 'darwin') throw new Error('原生横幅验收只支持 macOS');
const config = JSON.parse(readFileSync(join(root, 'mygo.json'), 'utf8'));
const arch = process.arch === 'x64' ? 'amd64' : process.arch;
const devBundle = join(root, '.mygo', 'dev', `darwin-${arch}`, `${config.name} Dev.app`);
const sourcePlist = join(devBundle, 'Contents', 'Info.plist');
try { readFileSync(sourcePlist); } catch {
  throw new Error('请先运行一次 make dev，以生成具备正确通知身份的 Dev 应用包');
}
const scratch = join(root, '.scratch', 'native-notifications');
mkdirSync(scratch, { recursive: true, mode: 0o700 });
const output = mkdtempSync(join(scratch, 'run-'));
const bundle = join(output, `${config.name} Dev.app`);
const contents = join(bundle, 'Contents');
mkdirSync(join(contents, 'MacOS'), { recursive: true });
copyFileSync(sourcePlist, join(contents, 'Info.plist'));
const resources = join(devBundle, 'Contents', 'Resources');
if (existsSync(resources)) cpSync(resources, join(contents, 'Resources'), { recursive: true });

function run(command, args, options = {}) {
  const result = spawnSync(command, args, { cwd: root, stdio: 'inherit', ...options });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`${command} exited with status ${result.status}`);
}
const executableResult = spawnSync('/usr/bin/plutil', ['-convert', 'json', '-o', '-', sourcePlist], { encoding: 'utf8' });
if (executableResult.status !== 0) throw new Error('无法读取 Dev 应用的可执行文件名称');
const info = JSON.parse(executableResult.stdout);
const executable = join(contents, 'MacOS', info.CFBundleExecutable);
const register = '/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister';
try {
  // Use the current icon as the test bundle's default so empty isolated config cannot restore an older icon.
  const managedIconKey = 'PageDockOriginalIcon';
  if (managedIconKey in info) run('/usr/bin/plutil', ['-remove', managedIconKey, join(contents, 'Info.plist')]);
  run('go', ['test', ...goTestFlags(), '-race', '-c', '-tags', 'desktop', '-o', executable, './internal']);
  run('codesign', ['--force', '--sign', '-', bundle]);
  console.log(`系统横幅证据：${output}`);
  run(executable, ['-test.run', '^TestDesktopNativeNotificationBanners$', '-test.v', '-test.timeout', '120s'], {
    cwd: join(root, 'internal'),
    env: { ...process.env, PAGEDOCK_TEST_NATIVE_NOTIFICATIONS: '1', PAGEDOCK_TEST_EVIDENCE_DIR: output },
  });
} finally {
  // Do not leave an executable test bundle registered under the same identity; retain resources for diagnosis.
  const removed = existsSync(executable) ? spawnSync(register, ['-u', bundle], { encoding: 'utf8' }) : null;
  try {
    renameSync(bundle, `${bundle.slice(0, -4)}.fixture`);
  } finally {
    const restored = spawnSync(register, ['-f', devBundle], { encoding: 'utf8' });
    if ((removed && (removed.error || removed.status !== 0)) || restored.error || restored.status !== 0) {
      console.error('测试包登记清理失败，请检查 LaunchServices；原测试结果见上方输出');
      process.exitCode = 1;
    }
  }
}
