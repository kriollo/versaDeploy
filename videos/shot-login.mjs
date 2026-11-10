// shot-login.mjs LOGIN_URL URL OUT.png — like shot.sh, after logging in (headless Chrome over
// the DevTools protocol, throwaway profile). EMAIL and PASS from the environment; the
// form is versaWYS's (#email, #password, #login).
import { spawn } from 'node:child_process';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { setTimeout as sleep } from 'node:timers/promises';

const [loginUrl, url, out] = process.argv.slice(2);
const profile = mkdtempSync(join(tmpdir(), 'shot-'));
const PORT = 9334;
const chrome = spawn('google-chrome', ['--headless=new', '--disable-gpu', '--hide-scrollbars', '--no-first-run',
    `--remote-debugging-port=${PORT}`, `--user-data-dir=${profile}`, '--window-size=1600,900', 'about:blank'], { stdio: 'ignore' });
try {
    let targets;
    for (let i = 0; i < 50 && !targets; i++) {
        try { targets = await (await fetch(`http://127.0.0.1:${PORT}/json/list`)).json(); } catch { await sleep(200); }
    }
    const ws = new WebSocket(targets.find(t => t.type === 'page').webSocketDebuggerUrl);
    await new Promise(r => ws.addEventListener('open', r));
    let seq = 0;
    const pending = new Map();
    ws.addEventListener('message', ({ data }) => {
        const m = JSON.parse(data);
        if (m.id && pending.has(m.id)) { pending.get(m.id)(m.result ?? m.error); pending.delete(m.id); }
    });
    const send = (method, params = {}) => new Promise(r => { const id = ++seq; pending.set(id, r); ws.send(JSON.stringify({ id, method, params })); });
    const evaluate = async e => (await send('Runtime.evaluate', { expression: e, awaitPromise: true, returnByValue: true }))?.result?.value;
    await send('Emulation.setDeviceMetricsOverride', { width: 1600, height: 900, deviceScaleFactor: 1, mobile: false });
    await send('Page.navigate', { url: loginUrl });
    await sleep(3000);
    await evaluate(`document.querySelector('#email').value=${JSON.stringify(process.env.EMAIL)}; document.querySelector('#password').value=${JSON.stringify(process.env.PASS)}; document.querySelector('#login').click();`);
    for (let i = 0; i < 30 && (await evaluate('location.href')) === loginUrl; i++) await sleep(300);
    await send('Page.navigate', { url });
    await sleep(4000);
    const { data } = await send('Page.captureScreenshot', { format: 'png' });
    writeFileSync(out, Buffer.from(data, 'base64'));
} finally {
    chrome.kill();
    await sleep(300);
    rmSync(profile, { recursive: true, force: true });
}
