'use strict';
const {Then,When}=require('@cucumber/cucumber');
const assert=require('node:assert/strict');
const driver=require('./build-stops-on-repeated-failure__driver/run.js');
When('build-stops-on-repeated-failure: the scripted build runs with:',function(doc){this.repeatedResult=driver.run(JSON.parse(doc));});
Then('build-stops-on-repeated-failure: the build satisfies the expected result',function(){assert.ok(this.repeatedResult.ok,this.repeatedResult.text);});
