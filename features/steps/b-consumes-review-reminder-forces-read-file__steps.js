const {Then} = require('@cucumber/cucumber');
const fs = require('fs');
const cp = require('child_process');
const assert = require('assert');
const {observe} = require('./b-consumes-review-reminder-forces-read-file__observer');
Then('b-consumes-review-reminder-forces-read-file: both forced choices are recognized', function(){
  assert.doesNotThrow(() => observe(fs.readFileSync('build/k_review_verdict_strand_test.go','utf8')));
});
Then('b-consumes-review-reminder-forces-read-file: the reset regression passes', {timeout:60000}, function(){
  const command='go test ./build -run ^TestKReviewVerdictStrandReset$ -count=1 -v';
  const r=cp.spawnSync('go',['test','./build','-run','^TestKReviewVerdictStrandReset$','-count=1','-v'],{encoding:'utf8'});
  assert.strictEqual(r.status,0,`${command}: TestKReviewVerdictStrandReset failed or did not run\n${r.stdout}${r.stderr}`);
  assert.match(r.stdout,/--- PASS: TestKReviewVerdictStrandReset /,`${command}: TestKReviewVerdictStrandReset absent, skipped or ran zero times\n${r.stdout}${r.stderr}`);
});
