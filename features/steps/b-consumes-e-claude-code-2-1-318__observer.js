const assert = require('node:assert/strict');
const { spawnSync } = require('node:child_process');
const command = 'go test -json ./build -run ^TestContextExternalFirstPrompt$ -count=1';
function run() {
  return spawnSync('go', ['test', '-json', './build', '-run', '^TestContextExternalFirstPrompt$', '-count=1'], {encoding:'utf8', timeout:170000, maxBuffer:16*1024*1024});
}
function observe(result, names) {
  const events = result.stdout.split('\n').filter(x => x.startsWith('{')).map(x => JSON.parse(x));
  for (const name of names) {
    const detail = `${name}: ${command}; status ${result.status}\n${result.stdout}${result.stderr}`;
    assert.equal(events.filter(e => e.Test === name && e.Action === 'run').length, 1, `${detail}; absent, duplicate or zero runs`);
    assert.equal(events.filter(e => e.Test === name && e.Action === 'pass').length, 1, `${detail}; failed or skipped`);
  }
  assert.equal(result.status, 0, `${names.join(', ')}: ${command}\n${result.stdout}${result.stderr}`);
}
module.exports = {run, observe};
if (require.main === module) {
  const result = run();
  const names = ['TestContextExternalFirstPrompt/context_false', 'TestContextExternalFirstPrompt/context_true'];
  // Qualify against an actual passing runner transcript before corrupting it.
  observe(result, names);
  if (process.argv[2] === 'bad') {
    result.stdout = result.stdout.split('\n').filter(line => {
      if (!line.startsWith('{')) return true;
      const event = JSON.parse(line);
      return !(event.Test === names[1] && event.Action === 'pass');
    }).join('\n');
  }
  try { observe(result, names); console.log('observer accepted'); }
  catch (error) {
    if (!(error instanceof assert.AssertionError)) throw error;
    console.log('observer rejected');
    process.exitCode = 1;
  }
}
