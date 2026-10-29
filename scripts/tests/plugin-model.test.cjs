const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { join } = require('node:path');
const { test } = require('node:test');
const { createContext, runInContext } = require('node:vm');

const model = createContext({});
runInContext(readFileSync(join(__dirname, '../../plugin/Model.js'), 'utf8'), model);
const plain = value => JSON.parse(JSON.stringify(value));
const ready = model.versionResult('ptcgl-omarchy 0.4.0\n', 0);
const checks = () => ['Proton', 'prefix', 'Visual C++ runtime', 'game', 'browser callback']
  .map(name => ({ name, ok: true, detail: '/a path with spaces' }));

test('launcher compatibility distinguishes missing, unusable, old, and supported releases', () => {
  assert.equal(model.versionResult('', 44).state, 'missing');
  for (const [output, code] of [['', 126], ['ptcgl-omarchy 0.4.0', 124], ['other-app 9.0.0', 0], ['not a version', 0]])
    assert.equal(model.versionResult(output, code).state, 'unavailable');
  for (const version of ['0.2.0', '0.3.99', '0.4.0-beta.1'])
    assert.equal(model.versionResult(`ptcgl-omarchy ${version}`, 0).state, 'outdated');
  for (const version of ['0.4.0', '0.4.1', '0.5.0-rc.1', '0.5.0', '1.0.0'])
    assert.equal(model.versionResult(`ptcgl-omarchy ${version}`, 0).state, 'compatible');
});

test('diagnostics separate first setup from integration errors and accept a failing-check exit', () => {
  const report = checks();
  assert.equal(model.diagnosticResult(JSON.stringify(report), 0).state, 'ready');
  report[0].ok = false;
  assert.equal(model.diagnosticResult(JSON.stringify(report), 1).state, 'setup-needed');
  report[4].ok = false;
  assert.equal(model.diagnosticResult(JSON.stringify(report), 1).state, 'attention');
  assert.match(model.diagnosticResult(JSON.stringify(report), 1).detail, /Proton/);
});

test('malformed, partial, timed-out, and contradictory diagnostic results remain unavailable', () => {
  const report = checks();
  for (const output of ['', '{broken', 'null', '{}', '[]', '[{"name":"game","ok":true,"detail":""}]'])
    assert.equal(model.diagnosticResult(output, 0).state, 'unavailable');
  assert.equal(model.diagnosticResult(JSON.stringify(report), 124).state, 'unavailable');
  assert.equal(model.diagnosticResult(JSON.stringify(report), 1).state, 'unavailable');
  report[0].ok = false;
  assert.equal(model.diagnosticResult(JSON.stringify(report), 0).state, 'unavailable');
  report[0].ok = 'false';
  assert.equal(model.diagnosticResult(JSON.stringify(report), 1).state, 'unavailable');
  const duplicate = checks();
  duplicate.push(duplicate[0]);
  assert.equal(model.diagnosticResult(JSON.stringify(duplicate), 0).state, 'unavailable');
});

test('missing or incompatible launchers cannot run game and maintenance commands', () => {
  for (const info of [model.loading(), model.versionResult('', 44), model.versionResult('ptcgl-omarchy 0.3.0', 0)]) {
    const actions = model.actions(info, { state: 'unavailable' }, false);
    for (const action of actions) {
      if (['play', 'check', 'update', 'repair', 'doctor', 'game', 'config', 'cache'].includes(action.id))
        assert.equal(action.enabled, false);
    }
  }
  assert.equal(model.actions(ready, { state: 'setup-needed' }, false)[0].text, 'Set up & play');
  assert.equal(model.actions(ready, { state: 'attention' }, false)[0].id, 'doctor');
  assert.equal(model.actions(ready, { state: 'ready' }, true)[0].enabled, false);
});

test('action arguments preserve paths and put flags before folder positional arguments', () => {
  const binary = "/home/a user/'$HOME/.local/bin/ptcgl-omarchy";
  const helper = '/a plugin/scripts/plugin-install.sh';
  assert.deepEqual(plain(model.actionCommand('play', binary, helper)), [binary, 'app']);
  assert.deepEqual(plain(model.actionCommand('check', binary, helper)), [binary, 'update', '--check', '--gui']);
  assert.deepEqual(plain(model.actionCommand('config', binary, helper)), [binary, 'open', '--gui', 'config']);
  assert.deepEqual(plain(model.actionCommand('install', binary, helper)), ['omarchy', 'launch', 'terminal', 'bash', helper]);
  assert.deepEqual(plain(model.actionCommand('arbitrary command', binary, helper)), []);
  assert.equal(model.localPath('file:///tmp/a%20plugin/%25%23%C3%A9.sh'), '/tmp/a plugin/%#é.sh');
  assert.throws(() => model.localPath('https://example.com/install.sh'));
});

test('keyboard navigation skips disabled actions and wraps in both directions', () => {
  const actions = [{ enabled: true }, { enabled: false }, { enabled: true }];
  assert.equal(model.nextEnabled(actions, 0, 1), 2);
  assert.equal(model.nextEnabled(actions, 2, 1), 0);
  assert.equal(model.nextEnabled(actions, 0, -1), 2);
  assert.equal(model.nextEnabled(actions, -1, -1), 2);
  assert.equal(model.nextEnabled([{ enabled: false }], 0, 1), -1);
});

function status(overrides = {}) {
  return {
    schema_version: 1, launcher_version: '0.5.0-rc.1', readiness: 'ready', activity: 'idle',
    phase: 'idle', message: 'Ready to play.', runtime: 'GE-Proton', custom_runtime: false,
    installer: { freshness: 'not-checked' },
    capabilities: { play: true, maintain: true, diagnostics: true, folders: true }, ...overrides
  };
}

test('live status drives Play, Focus, setup, repair, and busy actions', () => {
  for (const [payload, state, label, enabled] of [
    [status(), 'ready', 'Play', true],
    [status({ readiness: 'setup-needed' }), 'setup-needed', 'Set up & play', true],
    [status({ readiness: 'repair-needed', capabilities: { play: false, maintain: true, diagnostics: true, folders: true } }), 'repair-needed', 'Repair game', true],
    [status({ activity: 'running', capabilities: { play: true, maintain: false, diagnostics: true, folders: true } }), 'running', 'Focus game', true],
    [status({ activity: 'starting', capabilities: { play: false, maintain: false, diagnostics: true, folders: true } }), 'starting', 'Starting game…', false],
    [status({ activity: 'repair', capabilities: { play: false, maintain: false, diagnostics: true, folders: true } }), 'repair', 'Repairing game…', false]
  ]) {
    const parsed = model.statusResult(JSON.stringify(payload), 0);
    assert.equal(parsed.state, state);
    const actions = model.actions(ready, parsed, false, { maintenance: true, advanced: true });
    assert.equal(actions[0].text, label);
    assert.equal(actions[0].enabled, enabled);
    for (const action of actions.filter(a => ['update', 'repair', 'runtime', 'install'].includes(a.id))) {
      if (payload.activity !== 'idle') assert.equal(action.enabled, false);
    }
  }
});

test('malformed, unknown-schema, and contradictory live status fails closed', () => {
  for (const payload of [null, {}, status({ schema_version: 2 }), status({ activity: 'running' }), status({ activity: 'unavailable' }),
    status({ capabilities: { play: true } }), status({ readiness: 'repair-needed' }), status({ activity: 'surprise' }),
    status({ progress: { phase: 'downloading', downloaded: -1 } })]) {
    const parsed = model.statusResult(JSON.stringify(payload), 0);
    assert.equal(parsed.state, 'unavailable');
    const actions = model.actions(ready, parsed, false, { maintenance: true, advanced: true });
    assert.equal(actions.find(a => a.id === 'update').enabled, false);
    assert.equal(actions.find(a => a.id === 'runtime').enabled, false);
  }
  assert.equal(model.statusResult('', 124).state, 'unavailable');
  const old = model.statusResult('', 2);
  assert.equal(old.state, 'outdated');
  assert.equal(model.actions(ready, old, false)[0].text, 'Upgrade launcher');
});

test('default companion has one primary action and collapsed secondary sections', () => {
  const parsed = model.statusResult(JSON.stringify(status()), 0);
  const collapsed = model.actions(ready, parsed, false);
  assert.deepEqual(plain(collapsed.map(a => a.id)), ['play', 'toggle:maintenance', 'toggle:folders', 'toggle:advanced']);
  assert.equal(model.actions(ready, parsed, false, { folders: true }).filter(a => ['game', 'config', 'cache'].includes(a.id)).length, 3);
});

test('download presentation distinguishes known and unknown totals', () => {
  const base = status({ activity: 'setup', capabilities: { play: false, maintain: false, diagnostics: true, folders: true } });
  base.progress = { phase: 'downloading', message: 'Downloading', downloaded: 1048576, total: 2097152, bytes_per_second: 1048576, cancellable: true };
  assert.match(model.statusResult(JSON.stringify(base), 0).detail, /50%/);
  base.progress.total = -1;
  const unknown = model.statusResult(JSON.stringify(base), 0).detail;
  assert.match(unknown, /total size unknown/);
  assert.doesNotMatch(unknown, /%/);
});

test('recovery and update commands are fixed argument arrays', () => {
  assert.deepEqual(plain(model.actionCommand('repair', '/a path/launcher', 'helper')), ['/a path/launcher', 'fix', '--gui', 'repair']);
  assert.deepEqual(plain(model.actionCommand('runtime', 'launcher', 'helper')), ['launcher', 'fix', '--gui', 'runtime-update']);
  assert.deepEqual(plain(model.actionCommand('updates', 'launcher', 'helper')), ['launcher', 'updates', '--gui']);
  assert.deepEqual(plain(model.actionCommand('toggle:maintenance', 'launcher', 'helper')), []);
});
